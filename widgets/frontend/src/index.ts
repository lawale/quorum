// Importing the components registers the custom elements as a side effect.
export { QuorumApprovalPanel } from './components/approval-panel';
export { QuorumRequestList } from './components/request-list';
export { QuorumStageProgress } from './components/stage-progress';

// Public API
export { configure } from './lib/config';
export type { GlobalConfig, MaybeGetter } from './lib/config';
export { createClient, ApiError } from './lib/api';
export type { ClientConfig, QuorumClient } from './lib/api';
export type {
  Request,
  RequestStatus,
  Approval,
  Policy,
  ApprovalStage,
  AuditLog,
  PaginatedResponse,
  ListResponse,
  ResolvedField,
  ResolvedItem,
  ResolvedDisplay,
} from './lib/types';
