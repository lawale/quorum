// Named views over the generated OpenAPI types. The shapes come from
// api/openapi.yaml, which the server is validated against, so nothing here is
// hand-maintained.
import type { components } from "./generated/schema.js";

type Schemas = components["schemas"];

export type QuorumRequest = Schemas["Request"];
export type RequestStatus = Schemas["RequestStatus"];
export type Approval = Schemas["Approval"];
export type Decision = Schemas["Decision"];
export type AuditLog = Schemas["AuditLog"];
export type CreateRequestInput = Schemas["CreateRequestInput"];

export type Policy = Schemas["Policy"];
export type PolicyInput = Schemas["PolicyInput"];
export type PolicyUpdate = Schemas["PolicyUpdate"];
export type ApprovalStage = Schemas["ApprovalStage"];
export type ScopeRule = Schemas["ScopeRule"];
export type BypassRule = Schemas["BypassRule"];
export type BypassCondition = Schemas["BypassCondition"];
export type BypassLimit = Schemas["BypassLimit"];
/** A Go duration string such as "24h", "1h30m" or "90s". */
export type Duration = Schemas["Duration"];
export type JSONObject = Schemas["JSONObject"];

export type Webhook = Schemas["Webhook"];
export type WebhookInput = Schemas["WebhookInput"];
export type WebhookEvent = Schemas["WebhookEvent"];
export type WebhookPayload = Schemas["WebhookPayload"];
export type ReviewRequest = Schemas["ReviewRequest"];
export type ResolvedScope = Schemas["ResolvedScope"];

export type Health = Schemas["Health"];

/** One page of a list endpoint. */
export interface Page<T> {
  data: T[];
  total: number;
  page: number;
}

/** Whether a request can still change. */
export function isTerminal(status: RequestStatus): boolean {
  return status !== "pending";
}
