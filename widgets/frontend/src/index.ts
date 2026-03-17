// Auto-register custom elements
import './components/ApprovalPanel.svelte';
import './components/RequestList.svelte';
import './components/StageProgress.svelte';

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
