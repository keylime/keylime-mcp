package keylime

import "net/http"

// Agent operational states
const (
	StateRegistered    = 0
	StateStart         = 1
	StateSaved         = 2
	StateGetQuote      = 3
	StateGetQuoteRetry = 4
	StateProvideV      = 5
	StateProvideVRetry = 6
	StateFailed        = 7
	StateTerminated    = 8
	StateInvalidQuote  = 9
	StateTenantFailed  = 10
)

var stateRepresentations = map[int]string{
	StateRegistered:    "Registered",
	StateStart:         "Start",
	StateSaved:         "Saved",
	StateGetQuote:      "Get Quote",
	StateGetQuoteRetry: "Get Quote (retry)",
	StateProvideV:      "Provide V",
	StateProvideVRetry: "Provide V (retry)",
	StateFailed:        "Failed",
	StateTerminated:    "Terminated",
	StateInvalidQuote:  "Invalid Quote",
	StateTenantFailed:  "Tenant Quote Failed",
}

func StateToString(state int) string {
	if str, ok := stateRepresentations[state]; ok {
		return str
	}
	return "Unknown"
}

type Config struct {
	VerifierURL    string
	RegistrarURL   string
	CertDir        string
	TLSEnabled     bool
	IgnoreHostname bool

	APIVersion string
	ClientCert string
	ClientKey  string
	CAPath     string
	Port       string
}

type KeylimeClient struct {
	baseURL    string
	apiVersion string
	httpClient *http.Client
}

type GetAllAgentsInput struct{}

type GetAllAgentsOutput struct {
	Agents []string `json:"agents"`
}

type KeylimeAgentListResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		UUIDs []string `json:"uuids"`
	} `json:"results"`
}

type KeylimeAgentStatusResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		OperationalState          int      `json:"operational_state"`
		V                         string   `json:"v"`
		IP                        string   `json:"ip"`
		Port                      int      `json:"port"`
		TPMPolicy                 string   `json:"tpm_policy"`
		VTPMPolicy                string   `json:"vtpm_policy"`
		MetaData                  string   `json:"meta_data"`
		HasMbRefstate             int      `json:"has_mb_refstate"`
		HasRuntimePolicy          int      `json:"has_runtime_policy"`
		AcceptTPMHashAlgs         []string `json:"accept_tpm_hash_algs"`
		AcceptTPMEncryptionAlgs   []string `json:"accept_tpm_encryption_algs"`
		AcceptTPMSigningAlgs      []string `json:"accept_tpm_signing_algs"`
		HashAlg                   string   `json:"hash_alg"`
		EncAlg                    string   `json:"enc_alg"`
		SignAlg                   string   `json:"sign_alg"`
		VerifierID                string   `json:"verifier_id"`
		VerifierIP                string   `json:"verifier_ip"`
		VerifierPort              int      `json:"verifier_port"`
		SeverityLevel             *int     `json:"severity_level"`
		LastEventID               *string  `json:"last_event_id"`
		AttestationCount          int      `json:"attestation_count"`
		LastReceivedQuote         *int     `json:"last_received_quote"`
		LastSuccessfulAttestation *int     `json:"last_successful_attestation"`
	} `json:"results"`
}

type GetFailedAgentsInput struct{}

type GetFailedAgentsOutput struct {
	FailedAgents []GetAgentStatusOutput `json:"failed_agents"`
}

type GetAgentStatusInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type GetAgentStatusOutput struct {
	AgentUUID                   string  `json:"agent_uuid"`
	OperationalState            int     `json:"operational_state"`
	OperationalStateDescription string  `json:"operational_state_description"`
	AttestationCount            int     `json:"attestation_count"`
	LastReceivedQuote           *int    `json:"last_received_quote,omitempty"`
	LastSuccessfulAttestation   *int    `json:"last_successful_attestation,omitempty"`
	SeverityLevel               *int    `json:"severity_level,omitempty"`
	LastEventID                 *string `json:"last_event_id,omitempty"`
	HashAlgorithm               string  `json:"hash_algorithm"`
	EncryptionAlgorithm         string  `json:"encryption_algorithm"`
	SigningAlgorithm            string  `json:"signing_algorithm"`
	VerifierID                  string  `json:"verifier_id"`
	VerifierAddress             string  `json:"verifier_address"`
	HasMeasuredBoot             bool    `json:"has_measured_boot"`
	HasRuntimePolicy            bool    `json:"has_runtime_policy"`
}

type ReactivateAgentInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type ReactivateAgentOutput struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Results struct{} `json:"results"`
}

type GetAgentPoliciesInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type GetAgentPoliciesOutput struct {
	AgentUUID                 string      `json:"agent_uuid"`
	TPMPolicy                 interface{} `json:"tpm_policy"`
	VTPMPolicy                interface{} `json:"vtpm_policy"`
	MetaData                  interface{} `json:"meta_data"`
	HasMeasuredBootPolicy     bool        `json:"has_measured_boot_policy"`
	HasRuntimePolicy          bool        `json:"has_runtime_policy"`
	AcceptedTPMHashAlgs       []string    `json:"accepted_tpm_hash_algs"`
	AcceptedTPMEncryptionAlgs []string    `json:"accepted_tpm_encryption_algs"`
	AcceptedTPMSigningAlgs    []string    `json:"accepted_tpm_signing_algs"`
}
