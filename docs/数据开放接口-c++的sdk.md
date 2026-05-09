# C++ SDK

- [整体流程](https://uat.rztcd.cn:89/api/doc#整体流程)
- [SDK使用说明](https://uat.rztcd.cn:89/api/doc#SDK使用说明)
- [错误码定义](https://uat.rztcd.cn:89/api/doc#错误码定义)
- [结果数据定义](https://uat.rztcd.cn:89/api/doc#结果数据定义)
- [接口说明](https://uat.rztcd.cn:89/api/doc#接口说明)
- SDK下载

### 整体流程

业务方通过政务微信提供的sdk，可以进行业务日志数据的获取。目前包括如下特性业务数据ID：

1. 90000031：登录；
2. 90000032：唤醒；
3. 90000033：访问应用；
4. 90000034：应用推送消息；
5. 90000035：发送消息；
6. 90000036：单聊聊天数据；
7. 90000037：群聊聊天数据；
8. 90000038：创建群；
9. 90000039：群加人；
10. 90000040：群踢人；
11. 90000041：退群；
12. 90000042：转让群主；
13. 90000043：解散群；
14. 90000044：群改名。

### SDK使用说明

SDK包含如下几个主要接口：

1. 初始化接口
2. 设置解密日志RSA私钥，支持路径和内容两种方式
3. 分页拉取业务日志接口
4. 下载图片、文件等资源接口

### 错误码定义

```
enum ErrCode{    ERR_INVALID_PARAM   = 10000,  //参数错误，请求参数错误    ERR_SOCKET_ERR      = 10001,  //网络错误，网络请求错误    ERR_PARSE_DATA_ERR  = 10002,  //数据解析失败    ERR_SYSTEM_ERR      = 10003,  //系统失败    ERR_ENC_KEY         = 10004,  //密钥错误导致加密失败    ERR_FILE_ID         = 10005,  //fileid错误    ERR_DECRYPT         = 10006,  //解密失败    ERR_MSG_PRIVATE_KEY = 10007,  //找不到消息加密版本的私钥，需要重新传入私钥对    ERR_PARSE_ENC_KEY   = 10008,  //解析encrypt_key出错    ERR_INVALID_IP      = 10009,  //ip非法    ERR_DATA_EXPIRE     = 10010,  //数据过期    ERR_COPR_ID         = 1000000,  //coprid错误    ERR_SECRECT         = 1000001,  //secrect错误};
```

### 结果数据定义

```
struct LogInfo {public:    LogInfo() : feature_id(0), log_time(0), idc(""), log_data("") {}    ~Loginfo() {}public:    unsigned int feature_id;    unsigned int log_time;    std::string idc;    std::string log_data;}struct LogMediaData {public:    LogMediaData() : is_finished(false) {}    ~LogMediaData() {}public:    std::string data;    bool is_finished;}
```

### 接口说明

- 初始化

第一步：

```
/**    获取SDK对象，首次使用初始化*    @param [in] url     本地部署政务微信openapi服务基地址*/WeWorkLocalSdk sdk(const std::string &url);
```

第二步：

```
/**    初始化函数*    @param [in] corpid         单位ID，格式如：wl33fd99d5c5，可以在政务微信管理站点--我的单位--单位信息查看*    @param [in] secret         数据开放服务应用的Secret，可以在政务微信管理站点--管理工具--数据开放服务查看*/int Init(const std::string &corpid, const std::string &secret);
```

- 设置解密日志RSA私钥文件路径或内容（如果不要求拉取数据并同时解密，则先无需设置私钥）
  两种方式：
  其一，按RSA私钥文件路径设置

```
/**    设置解密日志数据RSA私钥保存路径*    @param [in] rsa_pri_key_file     RSA私钥保存位置，绝对路径*/int SetRsaPrivateKeyPath(const std::string &rsa_pri_key_file);
```

其二，按RSA私钥文件内容设置

```
/**    设置解密日志数据RSA私钥内容*    @param [in] rsa_pri_key_data     RSA私钥内容*/int SetRsaPrivateKey(const std::string &rsa_pri_key_data);
```

- 分页拉取业务日志

```
/**    分页拉取业务日志*    @param [in] feature_id         业务数据ID，参看前述定义*    @param [in] start_time         起始时间，时间戳，拉取这个时间（包含）之后的日志数据*    @param [in] end_time         结束时间，时间戳，拉取截止到这个时间（包含）的日志数据，且必须和start_time在同一天*    @param [in] start_index     分页拉取的起始位置，首次拉取为0*    @param [in] limit             单次拉取日志最大条数限制，最大值不超过1000*    @param [out] log_list         日志列表*/int GetLogList(unsigned int feature_id,    unsigned long long start_time, unsigned long long end_time,    unsigned int start_index, unsigned int limit,    std::vector<LogInfo> &log_list);
```

说明：在[start_time, end_time]时间段内，判断拉取结束的条件是：log_list.size() < limit。

- 下载资源

```
/**    拉取日志中的媒体文件*    @param [in] file_id             聊天记录中记录的文件ID*    @param [in] start_index            分块获取文件的起始位置，首次设置为0*    @param [in] block_size            分块获取文件的大小，为0时获取全部*    @param [out] log_media_data        获取到的文件内容分块*/int GetLogMediaData(const std::string &file_id,    unsigned long long start_index, unsigned long long block_size,    LogMediaData &log_meda_data);
```

- 解密数据
  首先需要设置解密日志RSA私钥文件路径或内容。

```
/** * 解密日志内容和文件信息** @param [in]  enc_data             需要解密的数据* @param [in]  enc_key              解密key* @param [out] dec_data             解密的数据内容** @return 返回是否获取成功*      0   - 成功*      其他 - 失败*/int DecryptData(const string enc_data, string enc_key, string &dec_data);
```

### SDK下载

[下载c++ sdk](https://uat.rztcd.cn:89/wwopen/downloadfile/C++sdk.zip)