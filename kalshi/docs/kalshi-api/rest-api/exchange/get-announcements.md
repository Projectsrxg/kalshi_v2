# Get Exchange Announcements

```
GET /exchange/announcements
```

**Auth**: None

## Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `limit` | int | 100 | Results per page (1-100) |
| `cursor` | string | - | Pagination cursor |

## Response

| Field | Type | Description |
|-------|------|-------------|
| `announcements` | array | Announcement objects |
| `cursor` | string | Next page cursor |

### Announcement Object

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Announcement ID |
| `title` | string | Title |
| `message` | string | Full text |
| `type` | string | `maintenance`, `market`, `feature`, `general` |
| `status` | string | Status |
| `created_time` | string | ISO 8601 |
| `delivery_time` | string | ISO 8601 |
