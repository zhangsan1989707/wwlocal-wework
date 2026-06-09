package service

import (
	"container/list"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"wwlocal-wework/internal/crypto"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
	"wwlocal-wework/pkg/metrics"
)

type lruEntry struct {
	key   string
	value *crypto.RSADecryptor
}

type DecryptService struct {
	keyRepo   *repository.KeyRepository
	cache     map[string]*list.Element
	lruList   *list.List
	cacheMu   sync.RWMutex
	maxCache  int
}

func NewDecryptService(keyRepo *repository.KeyRepository) *DecryptService {
	return &DecryptService{
		keyRepo:  keyRepo,
		cache:    make(map[string]*list.Element),
		lruList:  list.New(),
		maxCache: 10,
	}
}

func (s *DecryptService) getDecryptor(version string) (*crypto.RSADecryptor, error) {
	if version == "" {
		return s.getActiveDecryptor()
	}

	s.cacheMu.RLock()
	if elem, ok := s.cache[version]; ok {
		s.lruList.MoveToFront(elem)
		dec := elem.Value.(*lruEntry).value
		s.cacheMu.RUnlock()
		return dec, nil
	}
	s.cacheMu.RUnlock()

	pemBytes, err := s.keyRepo.ReadKeyFile(version)
	if err != nil {
		return nil, fmt.Errorf("read key file failed: %w", err)
	}

	dec, err := crypto.NewRSADecryptor(string(pemBytes))
	if err != nil {
		return nil, fmt.Errorf("create RSA decryptor failed: %w", err)
	}

	s.cacheMu.Lock()
	// 如果已存在，移到前面
	if elem, ok := s.cache[version]; ok {
		s.lruList.MoveToFront(elem)
		elem.Value.(*lruEntry).value = dec
	} else {
		// 淘汰最久未使用的
		for s.lruList.Len() >= s.maxCache {
			oldest := s.lruList.Back()
			if oldest != nil {
				s.lruList.Remove(oldest)
				delete(s.cache, oldest.Value.(*lruEntry).key)
			}
		}
		elem := s.lruList.PushFront(&lruEntry{key: version, value: dec})
		s.cache[version] = elem
	}
	s.cacheMu.Unlock()
	return dec, nil
}

func (s *DecryptService) getActiveDecryptor() (*crypto.RSADecryptor, error) {
	activeKey, err := s.keyRepo.GetActive()
	if err != nil {
		return nil, fmt.Errorf("get active key failed: %w", err)
	}
	return s.getDecryptor(activeKey.Version)
}

func (s *DecryptService) decryptInternal(dec *crypto.RSADecryptor, item *model.WeWorkLogItem) (*model.LogEntry, error) {
	startTime := time.Now()
	
	encKeyBytes, err := dec.Decrypt(item.EncKey)
	if err != nil {
		metrics.RecordDecryption(false, time.Since(startTime))
		return nil, fmt.Errorf("RSA decrypt enc_key failed: %w", err)
	}

	aesDec, err := crypto.NewAESDecryptor(encKeyBytes)
	if err != nil {
		metrics.RecordDecryption(false, time.Since(startTime))
		return nil, fmt.Errorf("create AES decryptor failed: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(item.EncData)
	if err != nil {
		metrics.RecordDecryption(false, time.Since(startTime))
		return nil, fmt.Errorf("decode enc_data failed: %w", err)
	}

	plaintext, err := aesDec.Decrypt(ciphertext)
	if err != nil {
		metrics.RecordDecryption(false, time.Since(startTime))
		return nil, fmt.Errorf("AES decrypt failed: %w", err)
	}

	// 截去尾部8字节
	if len(plaintext) > 8 {
		plaintext = plaintext[:len(plaintext)-8]
	}

	var parsedJSON map[string]interface{}
	if err := json.Unmarshal(plaintext, &parsedJSON); err != nil {
		metrics.RecordDecryption(false, time.Since(startTime))
		return nil, fmt.Errorf("parse decrypted JSON failed: %w", err)
	}

	parsedJSONStr, _ := json.Marshal(parsedJSON)
	
	metrics.RecordDecryption(true, time.Since(startTime))

	return &model.LogEntry{
		FeatureID:  item.FeatureID,
		LogTime:    item.LogTime,
		IDC:        item.IDC,
		EncData:    item.EncData,
		EncKey:     item.EncKey,
		RawJSON:    string(plaintext),
		ParsedJSON: string(parsedJSONStr),
	}, nil
}

func (s *DecryptService) Decrypt(item *model.WeWorkLogItem) (*model.LogEntry, error) {
	dec, err := s.getActiveDecryptor()
	if err != nil {
		return nil, err
	}
	return s.decryptInternal(dec, item)
}

func (s *DecryptService) DecryptWithKey(item *model.WeWorkLogItem, version string) (*model.LogEntry, error) {
	dec, err := s.getDecryptor(version)
	if err != nil {
		return nil, err
	}
	return s.decryptInternal(dec, item)
}
