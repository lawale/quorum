<svelte:options customElement={{ tag: "quorum-request-list", shadow: "open" }} />

<script lang="ts">
  import { createClient, ApiError } from '../lib/api';
  import type { Request } from '../lib/types';
  import { timeAgo, getDisplay } from '../lib/utils';
  import StatusBadge from './internal/StatusBadge.svelte';

  let {
    'api-url': apiUrl = '',
    token = '',
    'auth-headers': authHeadersStr = '',
    status: statusFilter = '',
    type: typeFilter = '',
    'page-size': pageSizeStr = '10',
  }: {
    'api-url'?: string;
    token?: string;
    'auth-headers'?: string;
    status?: string;
    type?: string;
    'page-size'?: string;
  } = $props();

  let requests: Request[] = $state([]);
  let total = $state(0);
  let page = $state(1);
  let error: string | null = $state(null);
  let loading = $state(true);

  let pageSize = $derived(parseInt(pageSizeStr) || 10);
  let totalPages = $derived(Math.ceil(total / pageSize));

  function getClient() {
    let authHeaders: Record<string, string> | undefined;
    if (authHeadersStr) {
      try { authHeaders = JSON.parse(authHeadersStr); } catch {}
    }
    return createClient({ apiUrl, token: token || undefined, authHeaders });
  }

  function dispatch(name: string, detail: unknown) {
    const el = document.querySelector('quorum-request-list');
    el?.dispatchEvent(new CustomEvent(name, { detail, bubbles: true, composed: true }));
  }

  async function load() {
    if (!apiUrl) return;
    loading = true;
    error = null;
    try {
      const client = getClient();
      const res = await client.listRequests({
        status: statusFilter || undefined,
        type: typeFilter || undefined,
        page,
        per_page: pageSize,
      });
      requests = res.data;
      total = res.total;
    } catch (e) {
      error = e instanceof ApiError ? e.message : 'Failed to load';
      dispatch('quorum:error', { message: error, status: e instanceof ApiError ? e.status : 0 });
    } finally {
      loading = false;
    }
  }

  function selectRequest(req: Request) {
    dispatch('quorum:select', { requestId: req.id, request: req });
  }

  $effect(() => {
    if (apiUrl) {
      // Reset to page 1 when filters change
      page = 1;
      load();
    }
  });

  $effect(() => {
    // Reload when page changes (but not from the filter reset above)
    if (apiUrl && page > 0) load();
  });
</script>

<div class="list-container">
  {#if loading && requests.length === 0}
    <div class="loading">Loading requests...</div>
  {:else if error && requests.length === 0}
    <div class="error-box">{error}</div>
  {:else if requests.length === 0}
    <div class="empty">No requests found</div>
  {:else}
    <table class="table">
      <thead>
        <tr>
          <th>Type</th>
          <th>Status</th>
          <th>Maker</th>
          <th>Stage</th>
          <th>Created</th>
        </tr>
      </thead>
      <tbody>
        {#each requests as req}
          <tr class="row" onclick={() => selectRequest(req)}>
            <td class="type-cell">{getDisplay(req.metadata)?.title ?? req.type}</td>
            <td><StatusBadge status={req.status} /></td>
            <td class="maker-cell">{req.maker_id}</td>
            <td class="stage-cell">{req.current_stage + 1}</td>
            <td class="time-cell">{timeAgo(req.created_at)}</td>
          </tr>
        {/each}
      </tbody>
    </table>

    {#if totalPages > 1}
      <div class="pagination">
        <button class="page-btn" disabled={page <= 1} onclick={() => { page--; }}>Prev</button>
        <span class="page-info">{page} / {totalPages}</span>
        <button class="page-btn" disabled={page >= totalPages} onclick={() => { page++; }}>Next</button>
      </div>
    {/if}
  {/if}
</div>

<style>
  .list-container {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    border: 1px solid #e5e7eb;
    border-radius: 8px;
    background: #fff;
    color: #111827;
    overflow: hidden;
  }
  .loading, .error-box, .empty {
    font-size: 13px;
    padding: 24px 16px;
    text-align: center;
  }
  .loading { color: #6b7280; }
  .error-box { color: #ef4444; }
  .empty { color: #9ca3af; }
  .table {
    width: 100%;
    border-collapse: collapse;
    font-size: 13px;
  }
  th {
    text-align: left;
    padding: 8px 12px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: #6b7280;
    background: #f9fafb;
    border-bottom: 1px solid #e5e7eb;
  }
  td {
    padding: 10px 12px;
    border-bottom: 1px solid #f3f4f6;
  }
  .row {
    cursor: pointer;
    transition: background 0.1s;
  }
  .row:hover { background: #f9fafb; }
  .type-cell { font-family: monospace; font-size: 12px; }
  .maker-cell { color: #374151; }
  .stage-cell { color: #6b7280; text-align: center; }
  .time-cell { color: #9ca3af; white-space: nowrap; font-size: 12px; }
  .pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border-top: 1px solid #e5e7eb;
  }
  .page-btn {
    padding: 4px 12px;
    font-size: 12px;
    border: 1px solid #d1d5db;
    border-radius: 4px;
    background: #fff;
    color: #374151;
    cursor: pointer;
  }
  .page-btn:hover:not(:disabled) { background: #f3f4f6; }
  .page-btn:disabled { opacity: 0.4; cursor: not-allowed; }
  .page-info { font-size: 12px; color: #6b7280; }
</style>
