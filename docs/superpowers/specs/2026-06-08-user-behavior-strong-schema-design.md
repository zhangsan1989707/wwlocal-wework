# User Behavior Timeline With Strong Schema Log Tables

## Background

The current system stores decrypted WeWork open-data logs in monthly tables named
`log_{feature_id}_{YYYYMM}`. Those tables keep the raw decrypted payload in
`raw_json` and `parsed_json`, with a small set of generated OpenID columns for
common filters.

The next feature is a read-only user behavior timeline. The user chose a strong
schema approach and will resync data instead of migrating historical JSON-only
tables. RSA key storage remains unchanged: private keys stay in
`keys/{version}/rsa_private_key.pem`, optionally encrypted by `KEY_ENCRYPT_KEY`.

No message revoke, message delete, file delete, room dismiss, or member removal
operations are included in this scope.

## Goals

- Store newly synced logs in feature-specific monthly tables with structured
  columns for stable fields.
- Preserve `raw_json` and `parsed_json` in every log table for troubleshooting
  and detail display.
- Add a read-only behavior query API that finds all logs related to one OpenID
  across selected features and returns why each row matched.
- Add a frontend behavior query page with timeline-style results.
- Avoid historical data migration.
- Keep existing key management, sync controls, dashboard, and generic log query
  behavior working.

## Non-Goals

- No migration of existing `log_*` tables.
- No destructive cleanup of existing log tables.
- No change to RSA key storage.
- No intervention APIs or UI actions.
- No new role model in this iteration.
- No full-text search or content moderation workflow.

## Data Model

Monthly table names stay unchanged:

```text
log_90000031_202606
log_90000036_202606
log_90000037_202606
```

All feature tables share base columns:

```text
id BIGINT AUTO_INCREMENT PRIMARY KEY
feature_id INT NOT NULL
log_time BIGINT NOT NULL
idc VARCHAR(32)
enc_data TEXT
enc_key TEXT
raw_json TEXT
parsed_json JSON
enc_data_hash CHAR(32)
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

Each supported feature adds structured columns. Examples:

- `90000031` login:
  `login_user_openid`, `login_user_type`, `deviceid`, `devtype`,
  `login_time`, `login_type`, `cli_ver`, `access_ip`, `cli_ip`.
- `90000036` single chat:
  `sender_openid`, `sender_type`, `send_time`, `receiver_openid`,
  `receiver_type`, `msg_type`, `msgid`, `appinfo`.
- `90000037` group chat:
  `sender_openid`, `sender_type`, `send_time`, `receiver`, `chatid`,
  `msg_type`, `msgid`, `appinfo`, `name`.
- Group operations `90000038` through `90000044`:
  structured operator, target, chat, member-list, and operation-time columns
  according to each feature's stable payload fields.
- `90000047`, `90000048`, `90000054`, `90000055`, `90000058`, `90000059`:
  structured OpenID, device, client, timestamp, and status columns where
  documented and stable.

Features configured locally but not yet fully mapped, including
`90000061`, `90000062`, `90000063`, and `90000066`, use the base columns plus
`raw_json` and `parsed_json` first. They can receive structured columns later
when their payload schemas are confirmed.

Indexes:

- `idx_log_time (log_time)`
- `idx_feature_logtime (feature_id, log_time)`
- `uk_dedup (feature_id, log_time, enc_data_hash)`
- OpenID indexes on scalar structured OpenID columns such as
  `login_user_openid`, `sender_openid`, `receiver_openid`, `openid`,
  `oper_openid`, and `user_openid`.

Array-like fields such as `members`, `receiver`, `add_members`, and
`del_members` are stored as JSON text in the first iteration. Behavior query can
match them with JSON-aware SQL or Go-side verification after a bounded candidate
query.

## Sync Flow

The sync flow remains:

```text
WeWork API -> DecryptService -> LogRepository.BatchSave -> monthly log table
```

`DecryptService` continues to return `model.LogEntry` containing raw and parsed
JSON. `LogRepository` becomes responsible for:

1. Creating a feature-specific monthly table when needed.
2. Mapping `parsed_json` into structured column values for that feature.
3. Inserting the base columns, structured columns, and hash in one statement.
4. Falling back to base columns for unmapped features.

Mapper failures must not stop the whole sync when the raw JSON is valid. The row
should still be stored with base columns and JSON payload unless the database
insert itself fails.

Existing daily active stats extraction can continue from parsed JSON during this
iteration. It may be optimized to use structured columns later.

## Behavior Query API

Add:

```http
POST /api/v1/logs/behavior-query
```

Request:

```json
{
  "openid": "13800138000",
  "feature_ids": [90000031, 90000036, 90000037],
  "start_time": 1710000000,
  "end_time": 1710600000,
  "page": 1,
  "page_size": 50
}
```

Validation:

- `openid` is required.
- `start_time` and `end_time` are required.
- `end_time` must be greater than or equal to `start_time`.
- `page_size` defaults to 50 and is capped at 200.
- Time range is capped at 31 days for this first version.
- If `feature_ids` is empty, enabled sync features are used.

Response:

```json
{
  "total": 12,
  "page": 1,
  "page_size": 50,
  "data": [
    {
      "feature_id": 90000036,
      "feature_name": "单聊聊天数据",
      "log_time": 1710000100,
      "log_date": "2024-03-10 12:01:40",
      "matched_fields": [
        {
          "field": "sender_openid",
          "label": "发送人",
          "value": "13800138000"
        }
      ],
      "data": {
        "sender_openid": "13800138000",
        "receiver_openid": "13900139000",
        "msg_type": 1
      }
    }
  ]
}
```

Behavior matching uses a feature field map. Scalar columns are queried directly.
Array/list fields are verified before returning the row so `matched_fields` is
truthful.

## Frontend

Add a `BehaviorTimeline` page and route, for example `/behavior`.

Controls:

- OpenID or mobile input.
- Feature multi-select.
- Time range picker with short presets.
- Page size selector.
- Search and reset buttons.

Results:

- Timeline or table sorted by `log_time` descending.
- Columns: time, feature name, matched fields, summary.
- Expandable JSON detail.
- No intervention buttons.

The page should follow the existing Element Plus admin UI style and reuse the
existing API client, auth store, and feature list API.

## Compatibility

Existing tables may still have old JSON-only structure. This design does not
migrate or drop them. New strong schema table creation applies when sync creates
a table after the change.

If an old table already exists for a feature and month, the repository may add
missing structured columns idempotently. This is only a compatibility migration
for table shape, not historical data backfill.

The generic log query should keep working because base columns and `parsed_json`
remain available.

## Testing

Backend tests:

- Mapper tests for representative payloads:
  `90000031`, `90000036`, `90000037`, `90000038`, `90000039`, and an unmapped
  feature.
- Behavior field matching tests for scalar fields and JSON list fields.
- Request validation tests for missing OpenID, invalid time range, and page-size
  caps.

Frontend checks:

- TypeScript build passes.
- Behavior page renders with empty, loading, error, and result states.
- Long JSON detail does not break the layout.

Manual acceptance:

1. Start with an empty database or cleared `log_*` tables.
2. Add and activate the existing RSA key as before.
3. Run sync for selected features.
4. Confirm newly created monthly tables include structured columns.
5. Query one OpenID and verify results show matched fields.
6. Confirm existing log query and dashboard pages still load.
