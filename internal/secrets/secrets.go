package secrets

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/crypto/pbkdf2"
)

const (
	keyIterations = 100000
	keyLength     = 32
	saltLength    = 16
)

// Vault stores encrypted secrets for coderaft projects
type Vault struct {
	mu       sync.RWMutex
	path     string
	secrets  map[string]map[string]string // project -> key -> encrypted value
	salt     []byte
	unlocked bool
	key      []byte
}

// VaultData is the on-disk format
type VaultData struct {
	Version  int                          `json:"version"`
	Salt     string                       `json:"salt"`
	Projects map[string]map[string]string `json:"projects"`
}

// NewVault creates or loads a secrets vault
func NewVault() (*Vault, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	vaultPath := filepath.Join(home, ".coderaft", "secrets.vault.json")
	v := &Vault{
		path:    vaultPath,
		secrets: make(map[string]map[string]string),
	}

	if err := v.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return v, nil
}

// load reads the vault from disk
func (v *Vault) load() error {
	data, err := os.ReadFile(v.path)
	if err != nil {
		return err
	}

	var vd VaultData
	if err := json.Unmarshal(data, &vd); err != nil {
		return fmt.Errorf("failed to parse vault: %w", err)
	}

	if vd.Salt != "" {
		v.salt, err = base64.StdEncoding.DecodeString(vd.Salt)
		if err != nil {
			return fmt.Errorf("failed to decode salt: %w", err)
		}
	}

	v.secrets = vd.Projects
	if v.secrets == nil {
		v.secrets = make(map[string]map[string]string)
	}

	return nil
}

// save writes the vault to disk
func (v *Vault) save() error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	dir := filepath.Dir(v.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create secrets directory: %w", err)
	}

	vd := VaultData{
		Version:  1,
		Salt:     base64.StdEncoding.EncodeToString(v.salt),
		Projects: v.secrets,
	}

	data, err := json.MarshalIndent(vd, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize vault: %w", err)
	}

	if err := os.WriteFile(v.path, data, 0600); err != nil {
		return fmt.Errorf("failed to write vault: %w", err)
	}

	return nil
}

// Initialize sets up the vault with a master password
func (v *Vault) Initialize(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.salt = make([]byte, saltLength)
	if _, err := rand.Read(v.salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	v.key = pbkdf2.Key([]byte(password), v.salt, keyIterations, keyLength, sha256.New)
	v.unlocked = true

	return v.save()
}

// Unlock decrypts the vault with the master password
func (v *Vault) Unlock(password string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if len(v.salt) == 0 {
		return fmt.Errorf("vault not initialized, run 'coderaft secrets init' first")
	}

	v.key = pbkdf2.Key([]byte(password), v.salt, keyIterations, keyLength, sha256.New)
	v.unlocked = true

	return nil
}

// IsInitialized checks if the vault has been set up
func (v *Vault) IsInitialized() bool {
	return len(v.salt) > 0
}

// IsUnlocked checks if the vault is currently unlocked
func (v *Vault) IsUnlocked() bool {
	return v.unlocked
}

// Set stores an encrypted secret for a project
func (v *Vault) Set(project, key, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.unlocked {
		return fmt.Errorf("vault is locked, unlock first")
	}

	encrypted, err := v.encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	if v.secrets[project] == nil {
		v.secrets[project] = make(map[string]string)
	}
	v.secrets[project][key] = encrypted

	return v.save()
}

// Get retrieves and decrypts a secret for a project
func (v *Vault) Get(project, key string) (string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked {
		return "", fmt.Errorf("vault is locked, unlock first")
	}

	projectSecrets, ok := v.secrets[project]
	if !ok {
		return "", fmt.Errorf("no secrets for project '%s'", project)
	}

	encrypted, ok := projectSecrets[key]
	if !ok {
		return "", fmt.Errorf("secret '%s' not found in project '%s'", key, project)
	}

	return v.decrypt(encrypted)
}

// Remove deletes a secret from a project
func (v *Vault) Remove(project, key string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.secrets[project] == nil {
		return fmt.Errorf("no secrets for project '%s'", project)
	}

	if _, ok := v.secrets[project][key]; !ok {
		return fmt.Errorf("secret '%s' not found in project '%s'", key, project)
	}

	delete(v.secrets[project], key)
	if len(v.secrets[project]) == 0 {
		delete(v.secrets, project)
	}

	return v.save()
}

// List returns all secret keys for a project (not the values)
func (v *Vault) List(project string) []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	projectSecrets, ok := v.secrets[project]
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(projectSecrets))
	for k := range projectSecrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ListProjects returns all projects with secrets
func (v *Vault) ListProjects() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	projects := make([]string, 0, len(v.secrets))
	for p := range v.secrets {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	return projects
}

// GetAll returns all decrypted secrets for a project (for injection into containers)
func (v *Vault) GetAll(project string) (map[string]string, error) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.unlocked {
		return nil, fmt.Errorf("vault is locked, unlock first")
	}

	projectSecrets, ok := v.secrets[project]
	if !ok {
		return make(map[string]string), nil
	}

	result := make(map[string]string, len(projectSecrets))
	for k, encrypted := range projectSecrets {
		decrypted, err := v.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret '%s': %w", k, err)
		}
		result[k] = decrypted
	}

	return result, nil
}

// encrypt uses AES-GCM to encrypt a value
func (v *Vault) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt uses AES-GCM to decrypt a value
func (v *Vault) decrypt(encoded string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(v.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (wrong password?)")
	}

	return string(plaintext), nil
}

// LoadEnvFile parses a .env file and returns key-value pairs
func LoadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Handle export prefix
		line = strings.TrimPrefix(line, "export ")

		// Split on first =
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Remove surrounding quotes
		if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
			(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
			value = value[1 : len(value)-1]
		}

		env[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading .env file: %w", err)
	}

	return env, nil
}

// MergeEnvFiles loads multiple .env files and merges them (later files override earlier)
func MergeEnvFiles(paths ...string) (map[string]string, error) {
	merged := make(map[string]string)

	for _, path := range paths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		env, err := LoadEnvFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", path, err)
		}

		for k, v := range env {
			merged[k] = v
		}
	}

	return merged, nil
}

// GetProjectEnv returns merged environment variables for a project:
// 1. .env file (base)
// 2. .env.local file (local overrides)
// 3. Vault secrets (highest priority)
func GetProjectEnv(workspacePath, project string, vault *Vault) (map[string]string, error) {
	env := make(map[string]string)

	// Load .env files
	envFiles := []string{
		filepath.Join(workspacePath, ".env"),
		filepath.Join(workspacePath, ".env.local"),
	}

	fileEnv, err := MergeEnvFiles(envFiles...)
	if err != nil {
		return nil, err
	}
	for k, v := range fileEnv {
		env[k] = v
	}

	// Overlay vault secrets if vault is unlocked
	if vault != nil && vault.IsUnlocked() {
		vaultEnv, err := vault.GetAll(project)
		if err != nil {
			return nil, err
		}
		for k, v := range vaultEnv {
			env[k] = v
		}
	}

	return env, nil
}
