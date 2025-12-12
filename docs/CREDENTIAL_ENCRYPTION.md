# Credential Encryption

## Overview

MrRSS now supports automatic encryption of sensitive credentials stored in the database. This feature enhances security by encrypting API keys, passwords, and other sensitive information using AES-256-GCM encryption with machine-specific keys.

## Encrypted Fields

The following sensitive settings are automatically encrypted:

1. **Translation API Keys**
   - `deepl_api_key` - DeepL translation API key
   - `baidu_secret_key` - Baidu translation secret key
   - `ai_api_key` - AI translation API key

2. **Summary API Keys**
   - `summary_ai_api_key` - AI summary service API key

3. **Proxy Credentials**
   - `proxy_username` - Proxy authentication username
   - `proxy_password` - Proxy authentication password

## How It Works

### Encryption Method

- **Algorithm**: AES-256-GCM (Galois/Counter Mode)
- **Key Derivation**: PBKDF2 with 100,000 iterations
- **Key Source**: Machine-specific identifier (hostname + OS + architecture)
- **Salt**: Random 16-byte salt (stored with ciphertext)
- **Nonce**: Random 12-byte nonce (stored with ciphertext)
- **Storage Format**: Base64-encoded `[salt(16 bytes)][nonce(12 bytes)][ciphertext+tag]`

### Automatic Migration

When upgrading from older versions with plain-text credentials:

1. **First Read**: On the first access to a plain-text credential:
   - The system detects the value is not encrypted
   - Automatically encrypts the value
   - Stores the encrypted version back to the database
   - Returns the original plain-text value to the application
   - Logs the migration: `Migrating plain text setting to encrypted: <key>`

2. **Subsequent Reads**: All future reads will decrypt and return the value seamlessly

3. **Zero Downtime**: The migration happens transparently without user intervention

### Machine-Specific Encryption

- Encrypted data is tied to the specific machine using a machine-specific key
- **Portability**: When moving the database to a different machine:
  - Encrypted values cannot be decrypted on the new machine
  - On first read, the system will attempt decryption
  - If decryption fails, the system will log a warning and may need re-entry of credentials
  - This is a security feature to prevent unauthorized access to credentials

## For Developers

### Using Encrypted Settings

```go
// Reading encrypted settings
apiKey, err := db.GetEncryptedSetting("deepl_api_key")
if err != nil {
    // Handle error
}

// Writing encrypted settings
err := db.SetEncryptedSetting("deepl_api_key", "your-api-key")
if err != nil {
    // Handle error
}
```

### Testing Encryption

Run the comprehensive test suite:

```bash
# Test encryption module
go test -v ./internal/crypto/

# Test database encryption
go test -v ./internal/database/ -run TestEncrypted

# Test migration
go test -v ./internal/database/ -run TestMigration
```

### Adding New Encrypted Fields

To encrypt additional settings:

1. Update database access to use `GetEncryptedSetting()` and `SetEncryptedSetting()`
2. Update all code that reads/writes the setting
3. The migration will happen automatically on first read

## Security Considerations

### Strengths

- ✅ **Strong Encryption**: AES-256-GCM provides authenticated encryption
- ✅ **Random Salt/Nonce**: Each encryption uses unique random values
- ✅ **Key Derivation**: PBKDF2 with 100k iterations strengthens the key
- ✅ **Machine-Bound**: Credentials are tied to the specific machine
- ✅ **Automatic Migration**: Seamless upgrade from plain-text

### Limitations

- ⚠️ **Machine Portability**: Encrypted data cannot be moved between machines
- ⚠️ **Key Management**: The encryption key is derived from machine properties
- ⚠️ **Physical Access**: An attacker with physical access to the machine could potentially derive the key

### Best Practices

1. **Backup**: Keep secure backups of your credentials before migrating databases
2. **Re-entry**: Be prepared to re-enter credentials when moving to a new machine
3. **Access Control**: Protect physical access to the machine running MrRSS
4. **Regular Updates**: Keep MrRSS updated for security patches

## Migration Guide

### From Plain-Text (v1.2.14 and earlier)

1. **Automatic**: Simply upgrade to the new version
2. **No Action Required**: Migration happens on first use
3. **Monitor Logs**: Check logs for migration messages if needed
4. **Verify**: Test that all integrations (translation, proxy, etc.) still work

### To New Machine

If moving your database to a new machine:

1. **Export Credentials**: Note down your API keys and passwords before moving
2. **Move Database**: Copy the database file to the new machine
3. **First Run**: Launch MrRSS on the new machine
4. **Re-enter Credentials**: If decryption fails, re-enter credentials in settings
5. **Test**: Verify all features work correctly

## Troubleshooting

### Decryption Errors

If you see decryption errors:

```
failed to decrypt setting <key>: decryption failed
```

**Solution**: Re-enter the credential in the application settings.

### Migration Not Working

If old credentials don't work after upgrade:

1. Check logs for migration messages
2. Verify the credential value in settings
3. Try clearing and re-entering the credential

### Database Moved Between Machines

If credentials don't work after moving database:

1. This is expected behavior (security feature)
2. Re-enter all sensitive credentials in settings
3. Test each integration (translation, proxy, etc.)

## Technical Details

### Encryption Implementation

Location: `internal/crypto/encryption.go`

Key functions:
- `Encrypt(plaintext string) (string, error)` - Encrypts a value
- `Decrypt(ciphertext string) (string, error)` - Decrypts a value
- `IsEncrypted(value string) bool` - Checks if a value is encrypted
- `GetMachineID() (string, error)` - Gets machine-specific identifier
- `DeriveKey(machineID string, salt []byte) []byte` - Derives encryption key

### Database Integration

Location: `internal/database/settings_db.go`

Key functions:
- `GetEncryptedSetting(key string) (string, error)` - Get and auto-migrate
- `SetEncryptedSetting(key, value string) error` - Encrypt and store

### Test Coverage

- ✅ Encryption/decryption roundtrip
- ✅ Multiple data types (API keys, passwords, unicode, JSON, URLs)
- ✅ Empty value handling
- ✅ Invalid input handling
- ✅ Key derivation consistency
- ✅ Migration from plain-text
- ✅ Migration idempotency
- ✅ Update/overwrite scenarios

## Version History

- **v1.2.15+**: Credential encryption with automatic migration
- **v1.2.14-**: Plain-text credential storage (deprecated)
