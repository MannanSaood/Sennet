# Phase 9 Tests

Test scripts and files for the Cross-Cloud Egress Auditor feature.

## Files

- `test_cloud_config.go` - Unit tests for cloud configuration and validation
- `test_api.ps1` - Integration tests for REST API endpoints

## Running Tests

### Go Unit Tests
```powershell
cd backend
go test ./cloud/... -v
go test ./correlation/... -v
```

### API Integration Tests
```powershell
.\tests\phase9\test_api.ps1
```
