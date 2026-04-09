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
	VerifierURL   string
	RegistrarURL  string
	CertDir       string
	TLSEnabled    bool
	TLSServerName string

	APIVersion string
	ClientCert string
	ClientKey  string // #nosec G117 -- field name for cert key path, value is not hardcoded
	CAPath     string
	Port       string
}

type Client struct {
	baseURL    string
	APIVersion string
	httpClient *http.Client
}

type GetAllAgentsInput struct{}

type GetAllAgentsOutput struct {
	Agents []string `json:"agents"`
}

type AgentListResponse struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		UUIDs []string `json:"uuids"`
	} `json:"results"`
}

type AgentStatusResponse struct {
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
	AgentUUID                 string   `json:"agent_uuid"`
	TPMPolicy                 any      `json:"tpm_policy"`
	VTPMPolicy                any      `json:"vtpm_policy"`
	MetaData                  any      `json:"meta_data"`
	HasMeasuredBootPolicy     bool     `json:"has_measured_boot_policy"`
	HasRuntimePolicy          bool     `json:"has_runtime_policy"`
	AcceptedTPMHashAlgs       []string `json:"accepted_tpm_hash_algs"`
	AcceptedTPMEncryptionAlgs []string `json:"accepted_tpm_encryption_algs"`
	AcceptedTPMSigningAlgs    []string `json:"accepted_tpm_signing_algs"`
}

type RegistrarGetAgentDetailsInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type RegistrarGetAgentDetailsOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		AikTpm   string `json:"aik_tpm"`
		EkTpm    string `json:"ek_tpm"`
		Ekcert   string `json:"ekcert"`
		MtlsCert string `json:"mtls_cert"`
		IP       string `json:"ip"`
		Port     int    `json:"port"`
		Regcount int    `json:"regcount"`
	} `json:"results"`
}

type GetAgentVersionInput struct{}

type GetAgentVersionOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		CurrentVersion    string   `json:"current_version"`
		SupportedVersions []string `json:"supported_versions"`
	} `json:"results"`
}

type RegistrarRemoveAgentInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type RegistrarRemoveAgentOutput struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Results struct{} `json:"results"`
}

type EnrollAgentToVerifierInput struct {
	AgentUUID         string `json:"agent_uuid"`
	RuntimePolicyName string `json:"runtime_policy_name"`
	MbPolicyName      string `json:"mb_policy_name"`
}

type EnrollAgentToVerifierOutput struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Results struct{} `json:"results"`
}

type UnenrollAgentFromVerifierInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type UnenrollAgentFromVerifierOutput struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Results struct{} `json:"results"`
}

type UpdateAgentInput struct {
	AgentUUID         string `json:"agent_uuid"`
	RuntimePolicyName string `json:"runtime_policy_name"`
	MbPolicyName      string `json:"mb_policy_name"`
}

type UpdateAgentOutput struct {
	AgentUUID string `json:"agent_uuid"`
	Status    string `json:"status"`
}

type StopAgentInput struct {
	AgentUUID string `json:"agent_uuid"`
}

type StopAgentOutput struct {
	Code    int      `json:"code"`
	Status  string   `json:"status"`
	Results struct{} `json:"results"`
}

type GetVerifierEnrolledAgentsInput struct{}

type GetVerifierEnrolledAgentsOutput struct {
	Agents []string `json:"agents"`
}

type ListRuntimePoliciesInput struct{}

type ListRuntimePoliciesOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		RuntimePolicyNames []string `json:"runtimepolicy names"`
	} `json:"results"`
}

type ImportRuntimePolicyInput struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
}

type ImportRuntimePolicyOutput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type GetRuntimePolicyInput struct {
	PolicyName string `json:"policy_name"`
}

type GetRuntimePolicyOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		Name          string `json:"name"`
		TPMPolicy     string `json:"tpm_policy"`
		RuntimePolicy string `json:"runtime_policy"`
	} `json:"results"`
}

type DeleteRuntimePolicyInput struct {
	PolicyName string `json:"policy_name"`
}

type DeletePolicyOutput struct {
	PolicyName string `json:"policy_name"`
	Status     string `json:"status"`
}

type ListMBPoliciesInput struct{}

type ListMBPoliciesOutput struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Results struct {
		MBPolicyNames []string `json:"mbpolicy names"`
	} `json:"results"`
}

type GetMBPolicyInput struct {
	PolicyName string `json:"policy_name"`
}

type GetMBPolicyOutput struct {
	Code    int            `json:"code"`
	Status  string         `json:"status"`
	Results map[string]any `json:"results"`
}

type ImportMBPolicyInput struct {
	Name     string `json:"name"`
	FilePath string `json:"file_path"`
}

type ImportMBPolicyOutput struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type DeleteMBPolicyInput struct {
	PolicyName string `json:"policy_name"`
}
