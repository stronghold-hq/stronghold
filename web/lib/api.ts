// Centralized API configuration and authenticated fetch wrapper

export const API_URL =
  process.env.NEXT_PUBLIC_API_URL ||
  (process.env.NODE_ENV === 'development'
    ? 'http://localhost:8080'
    : 'https://api.getstronghold.xyz');

/** Default request timeout in milliseconds */
const REQUEST_TIMEOUT_MS = 30_000;

/**
 * Creates a fetch call with an AbortController timeout.
 * If the caller already provides a signal, it is not overridden;
 * otherwise a 30-second timeout signal is attached.
 */
function fetchWithTimeout(
  input: string,
  init?: RequestInit
): Promise<Response> {
  // If the caller already set a signal, respect it
  if (init?.signal) {
    return fetch(input, init);
  }

  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

  return fetch(input, { ...init, signal: controller.signal }).finally(() => {
    clearTimeout(timeoutId);
  });
}

/**
 * Fetch wrapper that includes credentials and handles 401 with token refresh.
 * On 401, attempts to refresh the auth token and retries the original request once.
 * On refresh failure, redirects to the login page.
 * All requests have a 30-second timeout by default.
 */
export async function fetchWithAuth(
  input: string,
  init?: RequestInit
): Promise<Response> {
  const options: RequestInit = {
    ...init,
    credentials: 'include',
  };

  let response: Response;
  try {
    response = await fetchWithTimeout(input, options);
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') {
      throw new Error(`Request timed out after ${REQUEST_TIMEOUT_MS}ms`);
    }
    throw err;
  }

  if (response.status === 401) {
    // Attempt token refresh
    const refreshResponse = await fetchWithTimeout(`${API_URL}/v1/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
    });

    if (refreshResponse.ok) {
      // Retry the original request
      return fetchWithTimeout(input, options);
    }

    // Refresh failed - redirect to login and throw so callers
    // don't try to parse the stale 401 response as valid data.
    if (typeof window !== 'undefined') {
      window.location.href = '/dashboard/login';
    }
    throw new Error('Session expired');
  }

  return response;
}

// --- Typed API helpers ---

/** Balance information for a single wallet chain */
export interface WalletBalanceInfo {
  address: string;
  balance_usdc: string;
  network: string;
  error?: string;
}

/** Response from GET /v1/account/balances */
export interface BalancesResponse {
  evm?: WalletBalanceInfo;
  solana?: WalletBalanceInfo;
  total_usdc: string;
}

/**
 * Fetch on-chain USDC balances for the authenticated account's wallets.
 */
export async function fetchBalances(): Promise<BalancesResponse> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/balances`);
  if (!response.ok) {
    throw new Error('Failed to fetch balances');
  }
  return response.json();
}

// --- API Key types and helpers ---

/** An API key record (the raw key is only returned on creation) */
export interface APIKey {
  id: string;
  key_prefix: string;
  label: string;
  created_at: string;
  last_used_at: string | null;
  revoked_at: string | null;
}

/** Response from POST /v1/account/api-keys (includes the raw key once) */
export interface CreateAPIKeyResponse {
  key: string;
  id: string;
  key_prefix: string;
  label: string;
  created_at: string;
}

/**
 * Create a new API key with the given label.
 */
export async function createAPIKey(label: string): Promise<CreateAPIKeyResponse> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/api-keys`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ label }),
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: 'Failed to create API key' }));
    throw new Error(err.error || 'Failed to create API key');
  }
  return response.json();
}

/**
 * List all API keys for the authenticated account.
 */
export async function listAPIKeys(): Promise<APIKey[]> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/api-keys`);
  if (!response.ok) {
    throw new Error('Failed to list API keys');
  }
  const data = await response.json();
  return data.api_keys ?? [];
}

/**
 * Revoke an API key by ID.
 */
export async function revokeAPIKey(id: string): Promise<void> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/api-keys/${id}`, {
    method: 'DELETE',
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: 'Failed to revoke API key' }));
    throw new Error(err.error || 'Failed to revoke API key');
  }
}

// --- Account settings types and helpers ---

/** Account-level feature settings */
export interface AccountSettings {
  jailbreak_detection_enabled: boolean;
  has_api_keys: boolean;
}

/**
 * Get account settings.
 */
export async function getAccountSettings(): Promise<AccountSettings> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/settings`);
  if (!response.ok) {
    throw new Error('Failed to fetch account settings');
  }
  return response.json();
}

/**
 * Update account settings.
 */
export async function updateAccountSettings(
  settings: Partial<Pick<AccountSettings, 'jailbreak_detection_enabled'>>
): Promise<AccountSettings> {
  const response = await fetchWithAuth(`${API_URL}/v1/account/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  });
  if (!response.ok) {
    const err = await response.json().catch(() => ({ error: 'Failed to update settings' }));
    throw new Error(err.error || 'Failed to update settings');
  }
  return response.json();
}
