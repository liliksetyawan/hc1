# API Contracts - epic_requirement_1 (PROPOSED)

Rendering of `ifa-references.ir.json`. Confidence: 0.4 (greenfield; nothing verified against code).

## IFA registry
None. No IFA and no surrounding systems exist. `ifa_documents[]` and `surrounding_system_references[]` are intentionally empty.

## Internal API (workshop-registration-api, provenance: proposed)

| Endpoint | Method / Path (placeholder) | Status | Caller | AC |
|---|---|---|---|---|
| register | POST /api/registrations | new | workshop-registration-web | AC-WSR-001..005, 007, 009, 011 |
| availability | GET /api/availability | new | workshop-registration-web | AC-WSR-006, 008, 010, 012 |

`key-files.entry_points[]` is empty (no code); endpoints come from `code-notes.changes_required[]`. Paths/verbs are recommended placeholders, not declared.

## Error contract (DD-005 recommended default)

| HTTP | Code | Meaning |
|---|---|---|
| 201 | - | created |
| 422 (or 400) | VALIDATION_ERROR* | field-level errors |
| 409 | EMAIL_ALREADY_REGISTERED | duplicate normalised email |
| 409 | QUOTA_FULL | 30/30 taken |
| 500/503 | INTERNAL_ERROR* | generic |

*placeholder code string, not named in DD-005. Envelope `{"error":{"code","message","fields[]"}}` is inferred.

## Samples (all PROPOSED, `sample_source: proposed_from_ac_and_dd005`)

### Register - success (AC-WSR-001)
```http
POST /api/registrations
Content-Type: application/json

{ "name": "Karyawan A", "email": "a@corp.example" }
```
```json
201 { "registrationId": "REG-0029", "name": "Karyawan A", "email": "a@corp.example",
      "availability": { "total": 30, "registered": 29, "remaining": 1 } }
```
Error counterpart:
```json
500 { "error": { "code": "INTERNAL_ERROR", "message": "Terjadi gangguan sementara. Silakan coba lagi nanti." } }
```

### Register - duplicate email (AC-WSR-002, 011)
```json
POST body: { "name": "Budi", "email": " budi@corp.example " }
409 { "error": { "code": "EMAIL_ALREADY_REGISTERED", "message": "Email ini sudah terdaftar untuk workshop." } }
```

### Register - quota full (AC-WSR-003, 004, 005)
```json
POST body: { "name": "Karyawan D", "email": "d@corp.example" }
409 { "error": { "code": "QUOTA_FULL", "message": "Mohon maaf, kuota workshop sudah penuh." } }
```

### Register - validation error (AC-WSR-009)
```json
POST body: { "name": "", "email": "abc" }
422 { "error": { "code": "VALIDATION_ERROR", "message": "Data yang dimasukkan belum valid.",
      "fields": [ { "field": "name", "message": "Nama wajib diisi." },
                  { "field": "email", "message": "Format email tidak valid." } ] } }
```

### Availability (AC-WSR-006, 010)
```http
GET /api/availability
Accept: application/json
```
```json
200 { "total": 30, "registered": 12, "remaining": 18 }
503 { "error": { "code": "INTERNAL_ERROR", "message": "Layanan sementara tidak tersedia." } }
```

## Warnings / findings
- F-API-001 (medium): paths, verbs and envelope undeclared; ratify via DD-005.
- F-API-002 (low): no auth/header convention; authentication is out of scope per AC.
- F-API-003 (low): duplicate-vs-full precedence undecided; duplicate assumed first.
- 422 chosen over 400 as sample default; DD-005 allows either.
- No PII: all emails use `.example` placeholder domains.