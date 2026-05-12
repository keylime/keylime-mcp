# MCP Tools Reference

Tools exposed by the Keylime MCP server. Each tool maps to Keylime REST API operations.

## System

| Tool | Description |
|---|---|
| `Get_version_and_health` | Check Verifier and Registrar API versions and reachability |

## Agent Management

| Tool | Description |
|---|---|
| `Get_all_agents` | List all registered agent UUIDs from the Registrar |
| `Get_verifier_enrolled_agents` | List agent UUIDs enrolled in the Verifier |
| `Get_agent_status` | Get attestation state, quote history, and severity from the Verifier |
| `Get_agent_details` | Get hardware identity (EK, AIK, certs, IP) from the Registrar |
| `Get_agent_policies` | Get assigned TPM/vTPM policies and accepted algorithms |
| `Get_failed_agents` | List agents in a failed attestation state with failure details |
| `Get_verifier_logs` | Read Verifier journalctl logs, filterable by agent and failure type |
| `Enroll_agent_to_verifier` | Enroll a registered agent for active attestation with optional policies |
| `Update_agent` | Re-enroll an agent with new policies (safe unenroll + re-enroll) |
| `Unenroll_agent_from_verifier` | Remove an agent from the Verifier (keeps Registrar record) |
| `Reactivate_agent` | Resume attestation for a previously failed agent |
| `Stop_agent` | Pause Verifier polling without unenrolling |
| `Registrar_remove_agent` | Permanently delete an agent from the Registrar |

## Runtime Policies

| Tool | Description |
|---|---|
| `List_runtime_policies` | List policy names stored on the Verifier |
| `Get_runtime_policy` | Get a policy's digests, excludes, and keyrings |
| `Import_runtime_policy` | Upload a local policy JSON file to the Verifier |
| `Update_runtime_policy` | Add or remove excludes and digests in an existing policy |
| `Delete_runtime_policy` | Delete a policy from the Verifier |

## Measured Boot Policies

| Tool | Description |
|---|---|
| `List_mb_policies` | List measured boot policy names on the Verifier |
| `Get_mb_policy` | Get a policy's boot event logs and expected PCR values |
| `Import_mb_policy` | Upload a local measured boot policy JSON to the Verifier |
| `Delete_mb_policy` | Delete a measured boot policy from the Verifier |
