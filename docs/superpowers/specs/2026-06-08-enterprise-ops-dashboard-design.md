# Enterprise Operations Dashboard Delivery Design

## Goal

Bring the existing operations dashboard to a deliverable baseline for:

- admins or authorized users viewing only their management scope
- login users, device usage, usage users, active users, inactive users, and application usage metrics
- day, week, month, and quarter statistics
- CSV export using the same scoped data shown on screen

This design targets a focused enterprise baseline, not a full permission platform.

## Current Findings

The project already has real data foundations:

- WeWork log sync and decryption into monthly log tables
- contact sync into `contacts`, `departments`, and `contact_departments`
- `user_daily_stats` for daily user-feature activity
- Dashboard V2 precomputed tables and nightly calculation jobs
- Vue Dashboard V2 page and CSV export endpoints

Blocking gaps:

- authentication is single-account only and JWT only carries `username`
- no RBAC or department data scope
- Dashboard V2 frontend and backend response shapes do not match
- weekly/monthly/quarterly people metrics currently risk summing daily headcounts
- login users and scoped usage metrics are not complete enterprise metrics
- exports do not consistently cover all dashboard views

## Scope

In scope:

- basic multi-account auth
- two roles: `super_admin` and `dept_admin`
- department scope filtering, including child departments
- scoped Dashboard V2 overview, trend, department, device, user-detail, and export APIs
- minimal super-admin user management API and page for creating, disabling, and assigning department-scoped users
- corrected Dashboard V2 frontend API contract
- day/week/month/quarter trend controls
- aggregation rules that avoid duplicate people counts
- tests for permission scope and aggregation behavior

Out of scope:

- full menu/action permission management
- full self-service user administration beyond the minimum super-admin account/scope management screen
- per-application deep analysis unless existing log fields expose a stable app dimension
- historical data backfill UI
- external SSO or enterprise identity integration

## Permission Model

Add two tables:

- `users`
  - `id`
  - `username`
  - `password_hash`
  - `role`
  - `enabled`
  - timestamps
- `user_dept_scopes`
  - `user_id`
  - `dept_id`

Roles:

- `super_admin`: can view all data
- `dept_admin`: can view only assigned departments and their child departments

Startup compatibility:

- If `users` is empty, initialize a `super_admin` from existing `AUTH_USERNAME` and `AUTH_PASSWORD`.
- Existing deployments keep working after migration.

JWT claims:

- `user_id`
- `username`
- `role`
- `token_type`

Dashboard handlers must not trust frontend filters for authorization. Each scoped endpoint resolves the current user's effective department set server-side.

## User Management

Add a minimal super-admin-only management surface so the feature can be operated without direct database edits.

Backend endpoints:

- list users
- create user with role and password
- update user's role, enabled status, and department scopes
- reset user password

Frontend:

- add a compact user management page or section visible only to `super_admin`
- support creating `dept_admin` accounts
- support assigning one or more departments as scope roots
- support disabling users

This is not a full RBAC console. It only manages the fields required by this delivery.

## Scope Resolution

For `super_admin`:

- no department filter is applied

For `dept_admin`:

- load direct `dept_id` records from `user_dept_scopes`
- expand to child departments from `departments.parentid`
- apply this department set to all dashboard queries and exports

If a `dept_admin` has no department scope:

- return empty data, not global data

## Metric Definitions

People metrics use distinct users over the requested period:

- `login_users`: distinct users from feature `90000031`
- `usage_users`: distinct users from the active feature set
- `active`: same baseline as `usage_users` for this delivery
- `inactive`: scoped active contacts minus scoped usage users
- `activated`: distinct users from feature `90000048`
- `registered`: active contacts in scope

Event metrics use sum/count over the requested period:

- `msg_count`: rows from message features `90000035`, `90000036`, `90000037`
- `app_access_count`: rows from feature `90000033`
- `group_created`: rows from feature `90000038`
- `device_total`: distinct device users from feature `90000054`

Rates are recomputed from aggregated numerator and denominator:

- activation rate = activated users / registered users
- active rate = active users / registered users

Do not sum daily rates.

## Time Aggregation

Supported granularities:

- `day`
- `week`
- `month`
- `quarter`

People metrics:

- group by period
- count distinct users inside each period

Event metrics:

- group by period
- sum event rows inside each period

This avoids counting the same person multiple times in weekly, monthly, and quarterly people metrics.

## Dashboard V2 API Contract

Keep the existing route family:

- `GET /api/v1/dashboard/v2/overview`
- `GET /api/v1/dashboard/v2/trend`
- `GET /api/v1/dashboard/v2/multi-trend`
- `GET /api/v1/dashboard/v2/departments`
- `GET /api/v1/dashboard/v2/devices`
- `GET /api/v1/dashboard/v2/users`
- `GET /api/v1/dashboard/v2/export/overview`
- `GET /api/v1/dashboard/v2/export/users`

Add scoped export endpoints as needed:

- trend export
- department export
- device export

Response shapes:

Overview:

```json
{
  "date": "2026-06-07",
  "registered": 100,
  "activated": 90,
  "not_activated": 10,
  "login_users": 80,
  "usage_users": 75,
  "active": 75,
  "inactive": 25,
  "rate_activation": 900,
  "rate_active": 750,
  "msg_count": 1000,
  "msg_sender": 70,
  "group_created": 8,
  "group_active": 4,
  "app_access_user": 30,
  "app_access_count": 200,
  "devices": {
    "total": 60,
    "types": [
      { "type": "device_android", "name": "Android", "count": 20, "percentage": 33.3 }
    ]
  },
  "scope": {
    "role": "dept_admin",
    "dept_ids": [2, 3]
  }
}
```

Trend:

```json
{
  "granularity": "month",
  "periods": ["2026-06"],
  "series": {
    "login_users": [80],
    "usage_users": [75]
  }
}
```

Departments:

```json
[
  {
    "dept_id": 2,
    "dept_name": "Example",
    "total_contacts": 40,
    "active": 25,
    "inactive": 15,
    "active_rate": 62.5
  }
]
```

Devices:

```json
{
  "date": "2026-06-07",
  "total": 60,
  "types": [
    { "type": "device_ios", "name": "iOS", "count": 10, "percentage": 16.7 }
  ]
}
```

## Frontend Design

Keep the existing Dashboard V2 page.

Required changes:

- align TypeScript interfaces with backend response shapes
- use real metric keys such as `login_users`, `usage_users`, `active`, `inactive`, `msg_count`, and `app_access_count`
- add quarter to the trend granularity selector
- render scoped `devices.total` and `devices.types`
- consume department endpoint as an array
- show a subtle current scope label in the dashboard header
- ensure CSV exports call scoped backend endpoints

No new landing page or decorative redesign is planned.

## Export Design

Exports must use the same server-side scope as screen data.

Required exports:

- overview CSV
- trend CSV
- department CSV
- device CSV
- user-detail CSV

User-detail export supports list types:

- active
- inactive
- no_login

Use `no_login` consistently in backend and frontend.

## Error Handling

- Disabled users cannot log in.
- Invalid credentials return unauthorized.
- `dept_admin` with no scope receives empty dashboard data.
- Invalid date or granularity returns a validation error.
- Export endpoints return errors instead of silently exporting unscoped data.

## Testing

Backend tests:

- user initialization from existing auth config when `users` is empty
- password login for database users
- `super_admin` resolves unrestricted scope
- `dept_admin` resolves assigned departments plus children
- `dept_admin` with no scope resolves empty data
- people metric aggregation deduplicates users per period
- event metric aggregation sums counts per period
- Dashboard V2 service applies scope to overview, trends, users, departments, devices, and exports

Frontend checks:

- `npm run build`
- Dashboard V2 uses existing backend fields
- trend chart renders with `{periods, series}`
- devices render with `{total, types}`

General validation:

- `go test ./...`
- `npm run build`

## Acceptance Criteria

- A `super_admin` sees all departments and full totals.
- A `dept_admin` sees only assigned departments and child departments.
- Daily, weekly, monthly, and quarterly people metrics deduplicate users correctly.
- Event metrics remain summed over the selected period.
- Dashboard V2 panels do not read nonexistent fields.
- CSV exports match the same scoped data shown on screen.
- Existing single-admin deployment can upgrade without losing login access.
