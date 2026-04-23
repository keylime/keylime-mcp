package masking

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

func TestMaskUUIDs(t *testing.T) {
	e := NewEngine(true)

	text := `{"agents":["d432fbb3-d2f1-4a97-9ef7-75bd81c00000","a1b2c3d4-e5f6-7890-abcd-ef1234567890"]}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
	assert.NotContains(t, masked, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	assert.Contains(t, masked, "AGENT-1")
	assert.Contains(t, masked, "AGENT-2")
}

func TestMaskUUIDDeterministic(t *testing.T) {
	e := NewEngine(true)

	uuid := "d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
	text1 := `{"agent":"` + uuid + `"}`
	text2 := `{"other":"` + uuid + `"}`

	masked1 := e.Mask(text1)
	masked2 := e.Mask(text2)

	assert.Contains(t, masked1, "AGENT-1")
	assert.Contains(t, masked2, "AGENT-1")
}

func TestMaskIPv4(t *testing.T) {
	e := NewEngine(true)

	text := `{"ip":"192.168.1.100","verifier_ip":"10.0.0.5"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "192.168.1.100")
	assert.NotContains(t, masked, "10.0.0.5")
	assert.Contains(t, masked, "HOST-1")
	assert.Contains(t, masked, "HOST-2")
}

func TestMaskIPv4SkipsInvalid(t *testing.T) {
	e := NewEngine(true)

	text := `{"version":"999.999.999.999"}`
	masked := e.Mask(text)

	assert.Equal(t, text, masked)
}

func TestMaskHashes(t *testing.T) {
	e := NewEngine(true)

	sha256 := testSHA256
	sha1 := "da39a3ee5e6b4b0d3255bfef95601890afd80709"
	text := `{"digests":{"sha256":"` + sha256 + `","sha1":"` + sha1 + `"}}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, sha256)
	assert.NotContains(t, masked, sha1)
	assert.Contains(t, masked, "HASH-1")
	assert.Contains(t, masked, "HASH-2")
}

func TestMaskHashEquivalence(t *testing.T) {
	e := NewEngine(true)

	hash := testSHA256
	text := `{"allowlist":"` + hash + `","runtime":"` + hash + `"}`
	masked := e.Mask(text)

	count := strings.Count(masked, "HASH-1")
	assert.Equal(t, 2, count)
}

func TestMaskTPMBlobs(t *testing.T) {
	e := NewEngine(true)

	text := `{"aik_tpm":"long-aik-key-data","ek_tpm":"long-ek-key-data","ekcert":"cert-data","mtls_cert":"mtls-data","ip":"10.0.0.1"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "long-aik-key-data")
	assert.NotContains(t, masked, "long-ek-key-data")
	assert.NotContains(t, masked, "cert-data")
	assert.NotContains(t, masked, "mtls-data")
	assert.Contains(t, masked, `"aik_tpm":"TPM-1"`)
	assert.Contains(t, masked, `"ek_tpm":"TPM-2"`)
	assert.Contains(t, masked, `"ekcert":"TPM-3"`)
	assert.Contains(t, masked, `"mtls_cert":"TPM-4"`)
	assert.Contains(t, masked, "HOST-1")
}

func TestUnmask(t *testing.T) {
	e := NewEngine(true)

	uuid := "d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
	ip := "192.168.1.100"
	hash := testSHA256

	original := `{"uuid":"` + uuid + `","ip":"` + ip + `","hash":"` + hash + `"}`
	masked := e.Mask(original)

	assert.Contains(t, masked, "AGENT-1")
	assert.Contains(t, masked, "HOST-1")
	assert.Contains(t, masked, "HASH-1")

	unmasked := e.Unmask(masked)
	assert.Contains(t, unmasked, uuid)
	assert.Contains(t, unmasked, ip)
	assert.Contains(t, unmasked, hash)
}

func TestUnmaskUnknownAlias(t *testing.T) {
	e := NewEngine(true)

	text := `{"agent":"AGENT-99"}`
	unmasked := e.Unmask(text)

	assert.Equal(t, text, unmasked)
}

func TestDisabledEngine(t *testing.T) {
	e := NewEngine(false)

	text := `{"uuid":"d432fbb3-d2f1-4a97-9ef7-75bd81c00000","ip":"192.168.1.100"}`

	assert.Equal(t, text, e.Mask(text))
	assert.Equal(t, text, e.Unmask(text))
	assert.False(t, e.Enabled())
}

func TestMaskAgentStatusPayload(t *testing.T) {
	e := NewEngine(true)

	payload := map[string]any{
		"agent_uuid":                    "d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
		"operational_state":             3,
		"operational_state_description": "Get Quote",
		"attestation_count":             42,
		"hash_algorithm":                "sha256",
		"encryption_algorithm":          "rsa",
		"signing_algorithm":             "rsassa",
		"verifier_id":                   "default",
		"verifier_address":              "127.0.0.1:8881",
		"has_measured_boot":             false,
		"has_runtime_policy":            true,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	masked := e.Mask(string(data))

	assert.NotContains(t, masked, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
	assert.NotContains(t, masked, "127.0.0.1")
	assert.Contains(t, masked, "AGENT-1")
	assert.Contains(t, masked, "HOST-1")

	assert.Contains(t, masked, `"operational_state":3`)
	assert.Contains(t, masked, `"attestation_count":42`)
	assert.Contains(t, masked, `"hash_algorithm":"sha256"`)
}

func TestMaskRegistrarDetailsPayload(t *testing.T) {
	e := NewEngine(true)

	payload := map[string]any{
		"code":   200,
		"status": "Success",
		"results": map[string]any{
			"aik_tpm":   "test-aik-data-base64-encoded",
			"ek_tpm":    "test-ek-data-base64-encoded",
			"ekcert":    "test-ek-cert-pem",
			"mtls_cert": "test-mtls-cert-pem",
			"ip":        "192.168.1.100",
			"port":      9002,
			"regcount":  1,
		},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	masked := e.Mask(string(data))

	assert.NotContains(t, masked, "test-aik-data-base64-encoded")
	assert.NotContains(t, masked, "test-ek-data-base64-encoded")
	assert.NotContains(t, masked, "test-ek-cert-pem")
	assert.NotContains(t, masked, "test-mtls-cert-pem")
	assert.NotContains(t, masked, "192.168.1.100")

	assert.Contains(t, masked, `"aik_tpm":"TPM-1"`)
	assert.Contains(t, masked, `"ek_tpm":"TPM-2"`)
	assert.Contains(t, masked, `"ekcert":"TPM-3"`)
	assert.Contains(t, masked, `"mtls_cert":"TPM-4"`)
	assert.Contains(t, masked, "HOST-1")
}

func TestMaskMultipleAgentsPayload(t *testing.T) {
	e := NewEngine(true)

	payload := map[string]any{
		"agents": []string{
			"d432fbb3-d2f1-4a97-9ef7-75bd81c00000",
			"a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			"11111111-2222-3333-4444-555555555555",
		},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	masked := e.Mask(string(data))

	assert.Contains(t, masked, "AGENT-1")
	assert.Contains(t, masked, "AGENT-2")
	assert.Contains(t, masked, "AGENT-3")

	unmasked := e.Unmask(masked)
	assert.Contains(t, unmasked, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
	assert.Contains(t, unmasked, "a1b2c3d4-e5f6-7890-abcd-ef1234567890")
	assert.Contains(t, unmasked, "11111111-2222-3333-4444-555555555555")
}

func TestMaskRoundtrip(t *testing.T) {
	e := NewEngine(true)

	original := `{"uuid":"d432fbb3-d2f1-4a97-9ef7-75bd81c00000","ip":"10.0.0.1","hash":"` + testSHA256 + `"}`

	masked := e.Mask(original)
	assert.NotEqual(t, original, masked)

	unmasked := e.Unmask(masked)
	assert.Equal(t, original, unmasked)
}

func TestMaskPreservesNonSensitiveData(t *testing.T) {
	e := NewEngine(true)

	text := `{"operational_state":3,"hash_alg":"sha256","port":9002,"status":"Success","count":42}`
	masked := e.Mask(text)

	assert.Equal(t, text, masked)
}

func TestMaskErrorMessage(t *testing.T) {
	e := NewEngine(true)

	errMsg := "CRITICAL: agent d432fbb3-d2f1-4a97-9ef7-75bd81c00000 at 192.168.1.100 failed attestation"
	masked := e.Mask(errMsg)

	assert.NotContains(t, masked, "d432fbb3-d2f1-4a97-9ef7-75bd81c00000")
	assert.NotContains(t, masked, "192.168.1.100")
	assert.Contains(t, masked, "AGENT-1")
	assert.Contains(t, masked, "HOST-1")
	assert.Contains(t, masked, "CRITICAL")
	assert.Contains(t, masked, "failed attestation")
}

func TestMaskIPv6Full(t *testing.T) {
	e := NewEngine(true)

	text := `{"ip":"2001:0db8:85a3:0000:0000:8a2e:0370:7334"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "2001:0db8:85a3:0000:0000:8a2e:0370:7334")
	assert.Contains(t, masked, "HOST-1")
}

func TestMaskIPv6Compressed(t *testing.T) {
	e := NewEngine(true)

	text := `{"ip":"fe80::1","other":"2001:db8::8a2e:370:7334"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "fe80::1")
	assert.NotContains(t, masked, "2001:db8::8a2e:370:7334")
	assert.Contains(t, masked, "HOST-1")
	assert.Contains(t, masked, "HOST-2")
}

func TestMaskIPv6Loopback(t *testing.T) {
	e := NewEngine(true)

	text := `{"ip":"::1"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "::1")
	assert.Contains(t, masked, "HOST-1")
}

func TestMaskIPv6Roundtrip(t *testing.T) {
	e := NewEngine(true)

	original := `{"ip":"2001:db8::1"}`
	masked := e.Mask(original)
	unmasked := e.Unmask(masked)

	assert.Equal(t, original, unmasked)
}

func TestMaskIPv6AndIPv4Together(t *testing.T) {
	e := NewEngine(true)

	text := `{"ipv4":"192.168.1.1","ipv6":"fe80::1"}`
	masked := e.Mask(text)

	assert.NotContains(t, masked, "192.168.1.1")
	assert.NotContains(t, masked, "fe80::1")
	assert.Contains(t, masked, "HOST-1")
	assert.Contains(t, masked, "HOST-2")
}

func TestMaskIPv6SkipsNonIP(t *testing.T) {
	e := NewEngine(true)

	text := `{"hash_alg":"sha256","status":"ok"}`
	masked := e.Mask(text)

	assert.Equal(t, text, masked)
}

func TestMaskIMALogLine(t *testing.T) {
	e := NewEngine(true)

	imaLine := `10 7a84eba7af1c3e3aff7af83e2d4b945a1b1cb3e0 ima-ng sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 /usr/bin/test`
	masked := e.Mask(imaLine)

	assert.NotContains(t, masked, "7a84eba7af1c3e3aff7af83e2d4b945a1b1cb3e0")
	assert.NotContains(t, masked, testSHA256)
	assert.Contains(t, masked, "HASH-")
	assert.Contains(t, masked, "/usr/bin/test")
	assert.Contains(t, masked, "ima-ng")
	assert.Contains(t, masked, "sha256:")
}
