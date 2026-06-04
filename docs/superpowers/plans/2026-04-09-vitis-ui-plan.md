# Vitis UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a SvelteKit dashboard (`vitis-ui`) that consumes the Vitis Event API for browsing sessions/conversations, viewing turn timelines, monitoring live conversations, visualizing peer topology, and showing termination verdicts.

**Architecture:** SvelteKit app with Svelte 5 runes for reactive state, Svelte Flow for peer topology canvas, shadcn-svelte for UI components, and typed SSE/REST clients connecting to the `vitis serve` API.

**Tech Stack:** SvelteKit, Svelte 5, Svelte Flow (@xyflow/svelte), shadcn-svelte, Tailwind CSS 4, TypeScript

**Spec:** `docs/superpowers/specs/2026-04-09-vitis-event-api-and-ui-design.md`

**Prerequisite:** Vitis Event API plan must be completed first (`vitis serve` must be running).

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `vitis-ui/` (project root) | Create | SvelteKit project scaffold |
| `src/lib/api/types.ts` | Create | TypeScript models mirroring Go structs |
| `src/lib/api/rest.ts` | Create | Typed fetch wrappers for REST endpoints |
| `src/lib/api/sse.ts` | Create | EventSource wrapper with typed events + reconnect |
| `src/lib/api/health.ts` | Create | Health + status polling |
| `src/lib/stores/sessions.svelte.ts` | Create | Session/conversation list state |
| `src/lib/stores/conversation.svelte.ts` | Create | Active conversation state + turns + peer states |
| `src/lib/stores/connection.svelte.ts` | Create | SSE connection state |
| `src/lib/utils/format.ts` | Create | Time, ID truncation, confidence display |
| `src/lib/utils/peer-colors.ts` | Create | Deterministic color per peer ID |
| `src/lib/components/SessionCard.svelte` | Create | Session/conversation card |
| `src/lib/components/TurnCard.svelte` | Create | Turn timeline entry |
| `src/lib/components/PeerStatusCard.svelte` | Create | Peer status in live monitor |
| `src/lib/components/MetricsBar.svelte` | Create | Conversation progress metrics |
| `src/lib/components/TopologyCanvas.svelte` | Create | Svelte Flow peer graph |
| `src/lib/components/VerdictOverlay.svelte` | Create | Termination verdict modal |
| `src/lib/components/StatusBadge.svelte` | Create | Color-coded status badge |
| `src/lib/components/ConnectionIndicator.svelte` | Create | Sidebar connection dot |
| `src/routes/+layout.svelte` | Create | Shell with sidebar nav |
| `src/routes/+page.svelte` | Create | Redirect to /sessions |
| `src/routes/sessions/+page.svelte` | Create | Session browser |
| `src/routes/conversations/+page.svelte` | Create | Conversation browser |
| `src/routes/conversations/[id]/+page.svelte` | Create | Turn timeline |
| `src/routes/conversations/[id]/+page.ts` | Create | Load function for conversation data |
| `src/routes/conversations/[id]/live/+page.svelte` | Create | Live monitor |
| `src/routes/conversations/[id]/topology/+page.svelte` | Create | Peer topology canvas |

---

### Task 1: Project Scaffold

**Files:**
- Create: `vitis-ui/` project

- [ ] **Step 1: Create SvelteKit project**

```bash
cd /home/kamil-rybacki/Code
npx sv create vitis-ui --template minimal --types ts
cd vitis-ui
```

When prompted: select TypeScript, no additional options.

- [ ] **Step 2: Add Tailwind CSS**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx sv add tailwindcss
```

- [ ] **Step 3: Initialize shadcn-svelte**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx shadcn-svelte@latest init
```

When prompted: accept defaults (style: default, base color: slate, CSS variables: yes).

- [ ] **Step 4: Add required shadcn-svelte components**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx shadcn-svelte@latest add card badge dialog scroll-area separator button
```

- [ ] **Step 5: Install Svelte Flow**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npm install @xyflow/svelte
```

- [ ] **Step 6: Verify build**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npm run build
```

Expected: build succeeds

- [ ] **Step 7: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git init
git add -A
git commit -m "chore: scaffold SvelteKit project with shadcn-svelte, Tailwind, Svelte Flow"
```

---

### Task 2: TypeScript API Types

**Files:**
- Create: `src/lib/api/types.ts`

- [ ] **Step 1: Create types mirroring Go models**

```typescript
// src/lib/api/types.ts

export type RunStatus = 'spawning' | 'running' | 'completed' | 'errored' | 'timeout' | 'blocked';
export type ConversationStatus = 'pending' | 'running' | 'completed' | 'errored' | 'timeout' | 'sentinel' | 'max_turns';
export type TurnReason = 'opener' | 'addressed' | 'round_robin' | 'fallback';
export type VerdictDecision = 'continue' | 'complete' | 'timeout' | 'error';

export interface PeerSpec {
	readonly uri: string;
	readonly options?: Record<string, string>;
}

export interface PeerParticipant {
	readonly id: string;
	readonly spec: PeerSpec;
}

export interface TerminatorSpec {
	readonly kind: string;
	readonly sentinel?: string;
	readonly judge_uri?: string;
}

export interface Session {
	readonly session_id: string;
	readonly provider: string;
	readonly status: RunStatus;
	readonly started_at: string;
	readonly ended_at?: string;
	readonly duration_ms?: number;
	readonly exit_code?: number;
	readonly parser_confidence?: number;
	readonly observation_confidence?: number;
	readonly auth_mode: string;
	readonly blocked_reason?: string;
	readonly bytes_captured?: number;
	readonly warnings?: string[];
}

export interface Conversation {
	readonly conversation_id: string;
	readonly schema_version?: number;
	readonly created_at: string;
	readonly ended_at?: string;
	readonly status: ConversationStatus;
	readonly max_turns: number;
	readonly per_turn_timeout_sec: number;
	readonly overall_timeout_sec: number;
	readonly terminator: TerminatorSpec;
	readonly peers?: PeerParticipant[];
	readonly seeds?: Record<string, string>;
	readonly opener_id?: string;
	readonly peer_a: PeerSpec;
	readonly peer_b: PeerSpec;
	readonly seed_a: string;
	readonly seed_b: string;
	readonly opener: string;
	readonly turns_consumed: number;
	readonly reply_style?: string;
}

export interface ConversationTurn {
	readonly conversation_id: string;
	readonly index: number;
	readonly from: string;
	readonly from_id?: string;
	readonly to_id?: string;
	readonly reason?: TurnReason;
	readonly next_id_parsed?: string;
	readonly fallback_used?: boolean;
	readonly envelope: string;
	readonly response: string;
	readonly marker_token: string;
	readonly started_at: string;
	readonly ended_at: string;
	readonly completion_confidence: number;
	readonly parser_confidence: number;
	readonly warnings?: string[];
}

export interface Turn {
	readonly session_id: string;
	readonly turn_index: number;
	readonly role: string;
	readonly content: string;
	readonly created_at: string;
}

export interface Verdict {
	readonly conversation_id: string;
	readonly decision: VerdictDecision;
	readonly reason: string;
	readonly status: ConversationStatus;
}

export interface Envelope {
	readonly conversation_id: string;
	readonly turn_index: number;
	readonly max_turns: number;
	readonly from: string;
	readonly from_id?: string;
	readonly to_id?: string;
	readonly body: string;
	readonly marker_token: string;
	readonly include_briefing: boolean;
	readonly briefing?: string;
}

export interface ControlMsg {
	readonly conversation_id: string;
	readonly kind: string;
	readonly slot?: string;
	readonly reason?: string;
	readonly status?: ConversationStatus;
	readonly detail?: string;
	readonly verdict?: Verdict;
}

export interface HealthResponse {
	readonly status: string;
	readonly store: string;
}

export interface StatusResponse {
	readonly active_conversations: number;
	readonly active_sse_streams: number;
	readonly uptime_seconds: number;
	readonly store_backend: string;
	readonly version: string;
}

export interface ListResponse<T> {
	readonly data: T[];
	readonly total: number;
	readonly limit: number;
	readonly offset: number;
}

export interface ErrorResponse {
	readonly error: string;
	readonly detail: string;
}

// SSE event discriminated union
export type SSEEvent =
	| { type: 'turn'; data: ConversationTurn }
	| { type: 'envelope'; data: Envelope }
	| { type: 'control'; data: ControlMsg }
	| { type: 'conversation_started'; data: Conversation }
	| { type: 'conversation_ended'; data: Conversation }
	| { type: 'status_changed'; data: Conversation };
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/api/types.ts
git commit -m "feat: add TypeScript API types mirroring Vitis Go models"
```

---

### Task 3: REST Client

**Files:**
- Create: `src/lib/api/rest.ts`

- [ ] **Step 1: Implement typed REST client**

```typescript
// src/lib/api/rest.ts
import { PUBLIC_VITIS_API_URL } from '$env/static/public';
import type {
	Session,
	Conversation,
	ConversationTurn,
	Turn,
	ListResponse,
	HealthResponse,
	StatusResponse,
} from './types';

const BASE = PUBLIC_VITIS_API_URL ?? 'http://localhost:8090';

async function fetchJSON<T>(path: string, params?: Record<string, string>): Promise<T> {
	const url = new URL(path, BASE);
	if (params) {
		for (const [k, v] of Object.entries(params)) {
			if (v !== undefined && v !== '') url.searchParams.set(k, v);
		}
	}
	const resp = await fetch(url.toString());
	if (!resp.ok) {
		const body = await resp.json().catch(() => ({ error: 'unknown', detail: resp.statusText }));
		throw new Error(body.detail ?? resp.statusText);
	}
	return resp.json();
}

export function getHealth(): Promise<HealthResponse> {
	return fetchJSON('/health');
}

export function getStatus(): Promise<StatusResponse> {
	return fetchJSON('/api/v1/status');
}

export function listSessions(params?: {
	status?: string;
	limit?: number;
	offset?: number;
}): Promise<ListResponse<Session>> {
	return fetchJSON('/api/v1/sessions', {
		status: params?.status ?? '',
		limit: String(params?.limit ?? 20),
		offset: String(params?.offset ?? 0),
	});
}

export function getSession(id: string): Promise<Session> {
	return fetchJSON(`/api/v1/sessions/${encodeURIComponent(id)}`);
}

export function getSessionTurns(
	id: string,
	limit = 50,
): Promise<ListResponse<Turn>> {
	return fetchJSON(`/api/v1/sessions/${encodeURIComponent(id)}/turns`, {
		limit: String(limit),
	});
}

export function listConversations(params?: {
	status?: string;
	limit?: number;
	offset?: number;
}): Promise<ListResponse<Conversation>> {
	return fetchJSON('/api/v1/conversations', {
		status: params?.status ?? '',
		limit: String(params?.limit ?? 20),
		offset: String(params?.offset ?? 0),
	});
}

export function getConversation(id: string): Promise<Conversation> {
	return fetchJSON(`/api/v1/conversations/${encodeURIComponent(id)}`);
}

export function getConversationTurns(
	id: string,
	limit = 50,
): Promise<ListResponse<ConversationTurn>> {
	return fetchJSON(`/api/v1/conversations/${encodeURIComponent(id)}/turns`, {
		limit: String(limit),
	});
}
```

- [ ] **Step 2: Add env var to `.env`**

Create `/home/kamil-rybacki/Code/vitis-ui/.env`:

```
PUBLIC_VITIS_API_URL=http://localhost:8090
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/api/rest.ts .env
git commit -m "feat: add typed REST client for Vitis Event API"
```

---

### Task 4: SSE Client

**Files:**
- Create: `src/lib/api/sse.ts`

- [ ] **Step 1: Implement SSE client with reconnect**

```typescript
// src/lib/api/sse.ts
import { PUBLIC_VITIS_API_URL } from '$env/static/public';
import type { SSEEvent, ConversationTurn, Envelope, ControlMsg, Conversation } from './types';

const BASE = PUBLIC_VITIS_API_URL ?? 'http://localhost:8090';

type SSECallback = (event: SSEEvent) => void;
type StatusCallback = (status: 'connected' | 'reconnecting' | 'disconnected') => void;

export class VitisSSE {
	private es: EventSource | null = null;
	private url: string;
	private onEvent: SSECallback;
	private onStatus: StatusCallback;
	private retryCount = 0;
	private maxRetryDelay = 30_000;
	private retryTimer: ReturnType<typeof setTimeout> | null = null;

	constructor(url: string, onEvent: SSECallback, onStatus: StatusCallback) {
		this.url = url;
		this.onEvent = onEvent;
		this.onStatus = onStatus;
	}

	connect(): void {
		this.es = new EventSource(this.url);

		this.es.onopen = () => {
			this.retryCount = 0;
			this.onStatus('connected');
		};

		this.es.addEventListener('turn', (e) => {
			this.onEvent({ type: 'turn', data: JSON.parse(e.data) as ConversationTurn });
		});

		this.es.addEventListener('envelope', (e) => {
			this.onEvent({ type: 'envelope', data: JSON.parse(e.data) as Envelope });
		});

		this.es.addEventListener('control', (e) => {
			this.onEvent({ type: 'control', data: JSON.parse(e.data) as ControlMsg });
		});

		this.es.addEventListener('conversation_started', (e) => {
			this.onEvent({ type: 'conversation_started', data: JSON.parse(e.data) as Conversation });
		});

		this.es.addEventListener('conversation_ended', (e) => {
			this.onEvent({ type: 'conversation_ended', data: JSON.parse(e.data) as Conversation });
		});

		this.es.addEventListener('status_changed', (e) => {
			this.onEvent({ type: 'status_changed', data: JSON.parse(e.data) as Conversation });
		});

		this.es.onerror = () => {
			this.es?.close();
			this.onStatus('reconnecting');
			this.scheduleReconnect();
		};
	}

	disconnect(): void {
		if (this.retryTimer) clearTimeout(this.retryTimer);
		this.es?.close();
		this.es = null;
		this.onStatus('disconnected');
	}

	private scheduleReconnect(): void {
		const delay = Math.min(1000 * Math.pow(2, this.retryCount), this.maxRetryDelay);
		this.retryCount++;
		this.retryTimer = setTimeout(() => this.connect(), delay);
	}
}

export function createConversationStream(
	conversationId: string,
	onEvent: SSECallback,
	onStatus: StatusCallback,
): VitisSSE {
	const url = `${BASE}/api/v1/conversations/${encodeURIComponent(conversationId)}/stream`;
	const sse = new VitisSSE(url, onEvent, onStatus);
	sse.connect();
	return sse;
}

export function createLifecycleStream(
	onEvent: SSECallback,
	onStatus: StatusCallback,
): VitisSSE {
	const url = `${BASE}/api/v1/conversations/stream`;
	const sse = new VitisSSE(url, onEvent, onStatus);
	sse.connect();
	return sse;
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/api/sse.ts
git commit -m "feat: add SSE client with typed events and exponential backoff reconnect"
```

---

### Task 5: Health Polling + Connection Store

**Files:**
- Create: `src/lib/api/health.ts`
- Create: `src/lib/stores/connection.svelte.ts`

- [ ] **Step 1: Implement health polling**

```typescript
// src/lib/api/health.ts
import { getHealth, getStatus } from './rest';
import type { HealthResponse, StatusResponse } from './types';

type HealthCallback = (health: HealthResponse | null, status: StatusResponse | null) => void;

export function startHealthPolling(callback: HealthCallback, intervalMs = 10_000): () => void {
	let timer: ReturnType<typeof setInterval>;

	async function poll() {
		try {
			const [health, status] = await Promise.all([getHealth(), getStatus()]);
			callback(health, status);
		} catch {
			callback(null, null);
		}
	}

	poll();
	timer = setInterval(poll, intervalMs);

	return () => clearInterval(timer);
}
```

- [ ] **Step 2: Implement connection store**

```typescript
// src/lib/stores/connection.svelte.ts
import type { StatusResponse } from '$lib/api/types';

export type ConnectionState = 'connected' | 'reconnecting' | 'disconnected';

class ConnectionStore {
	state = $state<ConnectionState>('disconnected');
	apiStatus = $state<StatusResponse | null>(null);
	lastConnected = $state<Date | null>(null);
	reconnectAttempts = $state(0);

	setConnected(status: StatusResponse | null) {
		this.state = status ? 'connected' : 'disconnected';
		this.apiStatus = status;
		if (status) {
			this.lastConnected = new Date();
			this.reconnectAttempts = 0;
		}
	}

	setReconnecting() {
		this.state = 'reconnecting';
		this.reconnectAttempts++;
	}

	setDisconnected() {
		this.state = 'disconnected';
	}
}

export const connectionStore = new ConnectionStore();
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/api/health.ts src/lib/stores/connection.svelte.ts
git commit -m "feat: add health polling and connection state store"
```

---

### Task 6: Session and Conversation Stores

**Files:**
- Create: `src/lib/stores/sessions.svelte.ts`
- Create: `src/lib/stores/conversation.svelte.ts`

- [ ] **Step 1: Implement sessions store**

```typescript
// src/lib/stores/sessions.svelte.ts
import type { Session, Conversation, ConversationStatus, RunStatus } from '$lib/api/types';

class SessionsStore {
	sessions = $state<Session[]>([]);
	conversations = $state<Conversation[]>([]);
	statusFilter = $state<string>('');
	searchQuery = $state('');

	filteredSessions = $derived.by(() => {
		let result = this.sessions;
		if (this.statusFilter) {
			result = result.filter((s) => s.status === this.statusFilter);
		}
		if (this.searchQuery) {
			const q = this.searchQuery.toLowerCase();
			result = result.filter((s) => s.session_id.toLowerCase().includes(q));
		}
		return result;
	});

	filteredConversations = $derived.by(() => {
		let result = this.conversations;
		if (this.statusFilter) {
			result = result.filter((c) => c.status === this.statusFilter);
		}
		if (this.searchQuery) {
			const q = this.searchQuery.toLowerCase();
			result = result.filter((c) => c.conversation_id.toLowerCase().includes(q));
		}
		return result;
	});

	setSessions(sessions: Session[]) {
		this.sessions = sessions;
	}

	setConversations(conversations: Conversation[]) {
		this.conversations = conversations;
	}

	upsertConversation(conv: Conversation) {
		const idx = this.conversations.findIndex((c) => c.conversation_id === conv.conversation_id);
		if (idx >= 0) {
			this.conversations[idx] = conv;
		} else {
			this.conversations = [conv, ...this.conversations];
		}
	}
}

export const sessionsStore = new SessionsStore();
```

- [ ] **Step 2: Implement conversation detail store**

```typescript
// src/lib/stores/conversation.svelte.ts
import type { Conversation, ConversationTurn, Envelope, ControlMsg, Verdict } from '$lib/api/types';

export type PeerState = 'idle' | 'speaking' | 'done';

interface PeerInfo {
	id: string;
	state: PeerState;
	turnCount: number;
}

class ConversationStore {
	conversation = $state<Conversation | null>(null);
	turns = $state<ConversationTurn[]>([]);
	peers = $state<Map<string, PeerInfo>>(new Map());
	activeVerdict = $state<Verdict | null>(null);
	pinnedToBottom = $state(true);

	progress = $derived(
		this.conversation
			? Math.round((this.turns.length / this.conversation.max_turns) * 100)
			: 0,
	);

	activePeer = $derived.by(() => {
		for (const [, peer] of this.peers) {
			if (peer.state === 'speaking') return peer.id;
		}
		return null;
	});

	setConversation(conv: Conversation) {
		this.conversation = conv;
		this.initPeers(conv);
	}

	setTurns(turns: ConversationTurn[]) {
		this.turns = turns;
	}

	addTurn(turn: ConversationTurn) {
		this.turns = [...this.turns, turn];
		const fromId = turn.from_id ?? turn.from;
		this.updatePeerState(fromId, 'idle');
		this.incrementPeerTurnCount(fromId);
	}

	handleEnvelope(env: Envelope) {
		const toId = env.to_id ?? env.from;
		this.updatePeerState(toId, 'speaking');
	}

	handleControl(msg: ControlMsg) {
		if (msg.verdict) {
			this.activeVerdict = msg.verdict;
			for (const [id] of this.peers) {
				this.updatePeerState(id, 'done');
			}
		}
		if (msg.status && this.conversation) {
			this.conversation = { ...this.conversation, status: msg.status };
		}
	}

	dismissVerdict() {
		this.activeVerdict = null;
	}

	private initPeers(conv: Conversation) {
		const map = new Map<string, PeerInfo>();
		if (conv.peers) {
			for (const p of conv.peers) {
				map.set(p.id, { id: p.id, state: 'idle', turnCount: 0 });
			}
		} else {
			map.set('a', { id: 'a', state: 'idle', turnCount: 0 });
			map.set('b', { id: 'b', state: 'idle', turnCount: 0 });
		}
		this.peers = map;
	}

	private updatePeerState(peerId: string, state: PeerState) {
		const peer = this.peers.get(peerId);
		if (peer) {
			this.peers = new Map(this.peers).set(peerId, { ...peer, state });
		}
	}

	private incrementPeerTurnCount(peerId: string) {
		const peer = this.peers.get(peerId);
		if (peer) {
			this.peers = new Map(this.peers).set(peerId, { ...peer, turnCount: peer.turnCount + 1 });
		}
	}
}

export const conversationStore = new ConversationStore();
```

- [ ] **Step 3: Verify it compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 4: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/stores/sessions.svelte.ts src/lib/stores/conversation.svelte.ts
git commit -m "feat: add sessions and conversation reactive stores with Svelte 5 runes"
```

---

### Task 7: Utility Functions

**Files:**
- Create: `src/lib/utils/format.ts`
- Create: `src/lib/utils/peer-colors.ts`

- [ ] **Step 1: Implement formatters**

```typescript
// src/lib/utils/format.ts

export function truncateId(id: string, maxLen = 12): string {
	return id.length > maxLen ? id.slice(0, maxLen) + '...' : id;
}

export function formatDuration(ms: number): string {
	if (ms < 1000) return `${ms}ms`;
	const seconds = Math.floor(ms / 1000);
	if (seconds < 60) return `${seconds}s`;
	const minutes = Math.floor(seconds / 60);
	const remainingSeconds = seconds % 60;
	return `${minutes}m ${remainingSeconds}s`;
}

export function formatTimestamp(iso: string): string {
	const date = new Date(iso);
	return date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function formatConfidence(value: number): string {
	return `${Math.round(value * 100)}%`;
}

export function timeSince(iso: string): string {
	const ms = Date.now() - new Date(iso).getTime();
	return formatDuration(ms);
}
```

- [ ] **Step 2: Implement peer colors**

```typescript
// src/lib/utils/peer-colors.ts

const PALETTE = [
	'#818CF8', // indigo
	'#06B6D4', // cyan
	'#34D399', // emerald
	'#F59E0B', // amber
	'#F87171', // red
	'#E879F9', // purple
	'#FB923C', // orange
	'#38BDF8', // sky
] as const;

export function peerColor(peerId: string): string {
	let hash = 0;
	for (let i = 0; i < peerId.length; i++) {
		hash = ((hash << 5) - hash + peerId.charCodeAt(i)) | 0;
	}
	return PALETTE[Math.abs(hash) % PALETTE.length];
}

export function peerBgClass(peerId: string): string {
	const colors: Record<string, string> = {
		'#818CF8': 'bg-indigo-500/20 text-indigo-300',
		'#06B6D4': 'bg-cyan-500/20 text-cyan-300',
		'#34D399': 'bg-emerald-500/20 text-emerald-300',
		'#F59E0B': 'bg-amber-500/20 text-amber-300',
		'#F87171': 'bg-red-500/20 text-red-300',
		'#E879F9': 'bg-purple-500/20 text-purple-300',
		'#FB923C': 'bg-orange-500/20 text-orange-300',
		'#38BDF8': 'bg-sky-500/20 text-sky-300',
	};
	return colors[peerColor(peerId)] ?? 'bg-gray-500/20 text-gray-300';
}
```

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/utils/format.ts src/lib/utils/peer-colors.ts
git commit -m "feat: add format utilities and deterministic peer color assignment"
```

---

### Task 8: Shell Layout + Sidebar Navigation

**Files:**
- Create: `src/lib/components/ConnectionIndicator.svelte`
- Create: `src/routes/+layout.svelte`
- Create: `src/routes/+page.svelte`

- [ ] **Step 1: Create connection indicator component**

```svelte
<!-- src/lib/components/ConnectionIndicator.svelte -->
<script lang="ts">
	import { connectionStore } from '$lib/stores/connection.svelte';

	const stateColors: Record<string, string> = {
		connected: 'bg-green-500',
		reconnecting: 'bg-yellow-500 animate-pulse',
		disconnected: 'bg-red-500',
	};
</script>

<div class="flex items-center gap-2 text-xs text-muted-foreground">
	<span class="h-2 w-2 rounded-full {stateColors[connectionStore.state]}"></span>
	{#if connectionStore.state === 'connected' && connectionStore.apiStatus}
		<span>{connectionStore.apiStatus.active_conversations} active</span>
	{:else if connectionStore.state === 'reconnecting'}
		<span>Reconnecting ({connectionStore.reconnectAttempts})</span>
	{:else}
		<span>Disconnected</span>
	{/if}
</div>
```

- [ ] **Step 2: Create shell layout**

```svelte
<!-- src/routes/+layout.svelte -->
<script lang="ts">
	import '../app.css';
	import { startHealthPolling } from '$lib/api/health';
	import { connectionStore } from '$lib/stores/connection.svelte';
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import ConnectionIndicator from '$lib/components/ConnectionIndicator.svelte';

	let { children } = $props();

	$effect(() => {
		const stop = startHealthPolling((health, status) => {
			connectionStore.setConnected(status);
		});
		return stop;
	});

	const navItems = [
		{ href: '/sessions', label: 'Sessions' },
		{ href: '/conversations', label: 'Conversations' },
	];
</script>

<div class="flex h-screen bg-background text-foreground">
	<!-- Sidebar -->
	<aside class="flex w-56 flex-col border-r border-border bg-muted/30">
		<div class="p-4 text-lg font-semibold">Vitis</div>
		<nav class="flex-1 space-y-1 px-2">
			{#each navItems as item}
				<a
					href={item.href}
					class="block rounded-md px-3 py-2 text-sm hover:bg-muted"
				>
					{item.label}
				</a>
			{/each}
		</nav>
		<div class="border-t border-border p-3">
			<ConnectionIndicator />
		</div>
	</aside>

	<!-- Main -->
	<main class="flex-1 overflow-auto p-6">
		{@render children()}
	</main>

	<!-- Verdict overlay (global) -->
	{#if conversationStore.activeVerdict}
		{#await import('$lib/components/VerdictOverlay.svelte') then { default: VerdictOverlay }}
			<VerdictOverlay
				verdict={conversationStore.activeVerdict}
				ondismiss={() => conversationStore.dismissVerdict()}
			/>
		{/await}
	{/if}
</div>
```

- [ ] **Step 3: Create root redirect**

```svelte
<!-- src/routes/+page.svelte -->
<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	onMount(() => goto('/sessions'));
</script>
```

- [ ] **Step 4: Verify dev server runs**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npm run dev -- --port 5173
```

Expected: dev server starts, page loads with sidebar layout

- [ ] **Step 5: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/ConnectionIndicator.svelte src/routes/+layout.svelte src/routes/+page.svelte
git commit -m "feat: add shell layout with sidebar navigation and connection indicator"
```

---

### Task 9: StatusBadge + SessionCard Components

**Files:**
- Create: `src/lib/components/StatusBadge.svelte`
- Create: `src/lib/components/SessionCard.svelte`

- [ ] **Step 1: Create StatusBadge**

```svelte
<!-- src/lib/components/StatusBadge.svelte -->
<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';

	let { status }: { status: string } = $props();

	const variants: Record<string, string> = {
		running: 'bg-blue-500/20 text-blue-400 border-blue-500/30',
		completed: 'bg-green-500/20 text-green-400 border-green-500/30',
		errored: 'bg-red-500/20 text-red-400 border-red-500/30',
		timeout: 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
		pending: 'bg-gray-500/20 text-gray-400 border-gray-500/30',
		blocked: 'bg-orange-500/20 text-orange-400 border-orange-500/30',
		sentinel: 'bg-purple-500/20 text-purple-400 border-purple-500/30',
		max_turns: 'bg-amber-500/20 text-amber-400 border-amber-500/30',
	};
</script>

<Badge variant="outline" class={variants[status] ?? variants.pending}>
	{status}
</Badge>
```

- [ ] **Step 2: Create SessionCard**

```svelte
<!-- src/lib/components/SessionCard.svelte -->
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import StatusBadge from './StatusBadge.svelte';
	import { truncateId, formatDuration, formatTimestamp } from '$lib/utils/format';

	let { id, provider, status, startedAt, durationMs, turnCount, href }:
		{
			id: string;
			provider: string;
			status: string;
			startedAt: string;
			durationMs?: number;
			turnCount?: number;
			href: string;
		} = $props();
</script>

<a {href} class="block">
	<Card.Root class="transition-colors hover:bg-muted/50">
		<Card.Header class="flex flex-row items-center justify-between pb-2">
			<Card.Title class="text-sm font-mono">{truncateId(id)}</Card.Title>
			<StatusBadge {status} />
		</Card.Header>
		<Card.Content class="flex items-center gap-4 text-xs text-muted-foreground">
			<span>{provider}</span>
			{#if durationMs}
				<span>{formatDuration(durationMs)}</span>
			{/if}
			{#if turnCount !== undefined}
				<span>{turnCount} turns</span>
			{/if}
			<span>{formatTimestamp(startedAt)}</span>
		</Card.Content>
	</Card.Root>
</a>
```

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/StatusBadge.svelte src/lib/components/SessionCard.svelte
git commit -m "feat: add StatusBadge and SessionCard components"
```

---

### Task 10: Session & Conversation Browser (View 1)

**Files:**
- Create: `src/routes/sessions/+page.svelte`
- Create: `src/routes/conversations/+page.svelte`

- [ ] **Step 1: Create session browser page**

```svelte
<!-- src/routes/sessions/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { listSessions } from '$lib/api/rest';
	import { sessionsStore } from '$lib/stores/sessions.svelte';
	import SessionCard from '$lib/components/SessionCard.svelte';

	onMount(async () => {
		const resp = await listSessions({ limit: 50 });
		sessionsStore.setSessions(resp.data);
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="text-xl font-semibold">Sessions</h1>
		<input
			type="text"
			placeholder="Search by ID..."
			class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			oninput={(e) => (sessionsStore.searchQuery = e.currentTarget.value)}
		/>
	</div>

	<div class="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
		{#each sessionsStore.filteredSessions as session}
			<SessionCard
				id={session.session_id}
				provider={session.provider}
				status={session.status}
				startedAt={session.started_at}
				durationMs={session.duration_ms}
				href="/sessions"
			/>
		{/each}
	</div>

	{#if sessionsStore.filteredSessions.length === 0}
		<p class="text-center text-muted-foreground">No sessions found.</p>
	{/if}
</div>
```

- [ ] **Step 2: Create conversation browser page**

```svelte
<!-- src/routes/conversations/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { listConversations } from '$lib/api/rest';
	import { sessionsStore } from '$lib/stores/sessions.svelte';
	import { createLifecycleStream } from '$lib/api/sse';
	import SessionCard from '$lib/components/SessionCard.svelte';

	let sse: ReturnType<typeof createLifecycleStream> | null = null;

	onMount(async () => {
		const resp = await listConversations({ limit: 50 });
		sessionsStore.setConversations(resp.data);

		sse = createLifecycleStream(
			(event) => {
				if (event.type === 'conversation_started' || event.type === 'status_changed') {
					sessionsStore.upsertConversation(event.data);
				}
			},
			() => {},
		);

		return () => sse?.disconnect();
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<h1 class="text-xl font-semibold">Conversations</h1>
		<input
			type="text"
			placeholder="Search by ID..."
			class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
			oninput={(e) => (sessionsStore.searchQuery = e.currentTarget.value)}
		/>
	</div>

	<div class="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
		{#each sessionsStore.filteredConversations as conv}
			<SessionCard
				id={conv.conversation_id}
				provider={conv.peer_a.uri}
				status={conv.status}
				startedAt={conv.created_at}
				turnCount={conv.turns_consumed}
				href="/conversations/{conv.conversation_id}"
			/>
		{/each}
	</div>

	{#if sessionsStore.filteredConversations.length === 0}
		<p class="text-center text-muted-foreground">No conversations found.</p>
	{/if}
</div>
```

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/routes/sessions/+page.svelte src/routes/conversations/+page.svelte
git commit -m "feat: add Session and Conversation browser views"
```

---

### Task 11: TurnCard + Turn Timeline (View 2)

**Files:**
- Create: `src/lib/components/TurnCard.svelte`
- Create: `src/routes/conversations/[id]/+page.ts`
- Create: `src/routes/conversations/[id]/+page.svelte`

- [ ] **Step 1: Create TurnCard component**

```svelte
<!-- src/lib/components/TurnCard.svelte -->
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import Badge from '$lib/components/ui/badge/badge.svelte';
	import StatusBadge from './StatusBadge.svelte';
	import { peerBgClass } from '$lib/utils/peer-colors';
	import { formatConfidence, formatTimestamp } from '$lib/utils/format';
	import type { ConversationTurn } from '$lib/api/types';

	let { turn }: { turn: ConversationTurn } = $props();
	let expanded = $state(false);
</script>

<Card.Root class="border-l-2" style="border-left-color: var(--peer-color)">
	<Card.Header class="flex flex-row items-center gap-2 pb-2">
		<span class="rounded px-1.5 py-0.5 text-xs font-mono {peerBgClass(turn.from_id ?? turn.from)}">
			{turn.from_id ?? turn.from}
		</span>
		{#if turn.to_id}
			<span class="text-xs text-muted-foreground">-></span>
			<span class="rounded px-1.5 py-0.5 text-xs font-mono {peerBgClass(turn.to_id)}">
				{turn.to_id}
			</span>
		{/if}
		{#if turn.reason}
			<Badge variant="outline" class="text-xs">{turn.reason}</Badge>
		{/if}
		{#if turn.fallback_used}
			<Badge variant="outline" class="text-xs bg-yellow-500/20 text-yellow-400">fallback</Badge>
		{/if}
		<span class="ml-auto text-xs text-muted-foreground">{formatTimestamp(turn.started_at)}</span>
	</Card.Header>
	<Card.Content class="space-y-2">
		<button
			class="w-full text-left text-xs text-muted-foreground hover:text-foreground"
			onclick={() => (expanded = !expanded)}
		>
			{expanded ? 'Collapse' : 'Expand'} response ({turn.response.length} chars)
		</button>
		{#if expanded}
			<pre class="max-h-64 overflow-auto rounded bg-muted p-3 text-xs whitespace-pre-wrap">{turn.response}</pre>
		{/if}
		<div class="flex items-center gap-3 text-xs text-muted-foreground">
			<span>Completion: {formatConfidence(turn.completion_confidence)}</span>
			<span>Parser: {formatConfidence(turn.parser_confidence)}</span>
			{#if turn.warnings?.length}
				<span class="text-yellow-400">{turn.warnings.length} warning(s)</span>
			{/if}
		</div>
	</Card.Content>
</Card.Root>
```

- [ ] **Step 2: Create load function**

```typescript
// src/routes/conversations/[id]/+page.ts
import { getConversation, getConversationTurns } from '$lib/api/rest';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ params }) => {
	const [conversation, turnsResp] = await Promise.all([
		getConversation(params.id),
		getConversationTurns(params.id, 100),
	]);
	return { conversation, turns: turnsResp.data };
};
```

- [ ] **Step 3: Create turn timeline page**

```svelte
<!-- src/routes/conversations/[id]/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import { createConversationStream } from '$lib/api/sse';
	import TurnCard from '$lib/components/TurnCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { truncateId } from '$lib/utils/format';
	import { page } from '$app/state';
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();
	let timelineEl: HTMLDivElement;
	let sse: ReturnType<typeof createConversationStream> | null = null;

	onMount(() => {
		conversationStore.setConversation(data.conversation);
		conversationStore.setTurns(data.turns);

		if (data.conversation.status === 'running') {
			sse = createConversationStream(
				page.params.id,
				(event) => {
					if (event.type === 'turn') conversationStore.addTurn(event.data);
					if (event.type === 'envelope') conversationStore.handleEnvelope(event.data);
					if (event.type === 'control') conversationStore.handleControl(event.data);
				},
				() => {},
			);
		}

		return () => sse?.disconnect();
	});

	$effect(() => {
		if (conversationStore.pinnedToBottom && timelineEl) {
			timelineEl.scrollTop = timelineEl.scrollHeight;
		}
	});
</script>

<div class="space-y-4">
	<div class="flex items-center justify-between">
		<div class="flex items-center gap-3">
			<a href="/conversations" class="text-muted-foreground hover:text-foreground text-sm">&larr; Back</a>
			<h1 class="text-xl font-semibold font-mono">{truncateId(page.params.id)}</h1>
			{#if conversationStore.conversation}
				<StatusBadge status={conversationStore.conversation.status} />
			{/if}
		</div>
		{#if conversationStore.conversation?.status === 'running'}
			<div class="flex items-center gap-2">
				<a href="/conversations/{page.params.id}/live" class="text-sm text-blue-400 hover:underline">Live Monitor</a>
				<a href="/conversations/{page.params.id}/topology" class="text-sm text-blue-400 hover:underline">Topology</a>
			</div>
		{/if}
	</div>

	<div bind:this={timelineEl} class="max-h-[calc(100vh-12rem)] space-y-3 overflow-auto">
		{#each conversationStore.turns as turn (turn.index)}
			<TurnCard {turn} />
		{/each}
		{#if conversationStore.turns.length === 0}
			<p class="text-center text-muted-foreground">No turns yet.</p>
		{/if}
	</div>
</div>
```

- [ ] **Step 4: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/TurnCard.svelte src/routes/conversations/\[id\]/+page.ts src/routes/conversations/\[id\]/+page.svelte
git commit -m "feat: add TurnCard component and Turn Timeline view"
```

---

### Task 12: Live Conversation Monitor (View 3)

**Files:**
- Create: `src/lib/components/PeerStatusCard.svelte`
- Create: `src/lib/components/MetricsBar.svelte`
- Create: `src/routes/conversations/[id]/live/+page.svelte`

- [ ] **Step 1: Create PeerStatusCard**

```svelte
<!-- src/lib/components/PeerStatusCard.svelte -->
<script lang="ts">
	import * as Card from '$lib/components/ui/card';
	import { peerColor } from '$lib/utils/peer-colors';
	import type { PeerState } from '$lib/stores/conversation.svelte';

	let { id, state, turnCount }: { id: string; state: PeerState; turnCount: number } = $props();

	const stateLabels: Record<PeerState, string> = {
		idle: 'Idle',
		speaking: 'Speaking...',
		done: 'Done',
	};

	const stateColors: Record<PeerState, string> = {
		idle: 'border-muted',
		speaking: 'border-blue-500 animate-pulse',
		done: 'border-green-500',
	};
</script>

<Card.Root class="border-2 {stateColors[state]}">
	<Card.Header class="pb-1">
		<Card.Title class="flex items-center gap-2 text-sm">
			<span class="h-3 w-3 rounded-full" style="background-color: {peerColor(id)}"></span>
			<span class="font-mono">{id}</span>
		</Card.Title>
	</Card.Header>
	<Card.Content class="flex items-center justify-between text-xs text-muted-foreground">
		<span>{stateLabels[state]}</span>
		<span>{turnCount} turns</span>
	</Card.Content>
</Card.Root>
```

- [ ] **Step 2: Create MetricsBar**

```svelte
<!-- src/lib/components/MetricsBar.svelte -->
<script lang="ts">
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import { timeSince } from '$lib/utils/format';

	let elapsed = $state('');

	$effect(() => {
		const conv = conversationStore.conversation;
		if (!conv) return;
		const interval = setInterval(() => {
			elapsed = timeSince(conv.created_at);
		}, 1000);
		return () => clearInterval(interval);
	});
</script>

{#if conversationStore.conversation}
	<div class="flex items-center gap-6 rounded-lg bg-muted/50 px-4 py-2 text-sm">
		<div>
			<span class="text-muted-foreground">Elapsed: </span>
			<span class="font-mono">{elapsed}</span>
		</div>
		<div>
			<span class="text-muted-foreground">Turns: </span>
			<span class="font-mono">{conversationStore.turns.length} / {conversationStore.conversation.max_turns}</span>
		</div>
		<div class="flex-1">
			<div class="h-1.5 rounded-full bg-muted">
				<div
					class="h-full rounded-full bg-blue-500 transition-all"
					style="width: {conversationStore.progress}%"
				></div>
			</div>
		</div>
		{#if conversationStore.activePeer}
			<div>
				<span class="text-muted-foreground">Active: </span>
				<span class="font-mono text-blue-400">{conversationStore.activePeer}</span>
			</div>
		{/if}
	</div>
{/if}
```

- [ ] **Step 3: Create Live Monitor page**

```svelte
<!-- src/routes/conversations/[id]/live/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getConversation, getConversationTurns } from '$lib/api/rest';
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import { createConversationStream } from '$lib/api/sse';
	import PeerStatusCard from '$lib/components/PeerStatusCard.svelte';
	import MetricsBar from '$lib/components/MetricsBar.svelte';
	import TurnCard from '$lib/components/TurnCard.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { truncateId } from '$lib/utils/format';

	let sse: ReturnType<typeof createConversationStream> | null = null;
	let timelineEl: HTMLDivElement;

	onMount(async () => {
		const [conv, turnsResp] = await Promise.all([
			getConversation(page.params.id),
			getConversationTurns(page.params.id, 100),
		]);
		conversationStore.setConversation(conv);
		conversationStore.setTurns(turnsResp.data);

		sse = createConversationStream(
			page.params.id,
			(event) => {
				if (event.type === 'turn') conversationStore.addTurn(event.data);
				if (event.type === 'envelope') conversationStore.handleEnvelope(event.data);
				if (event.type === 'control') conversationStore.handleControl(event.data);
			},
			() => {},
		);

		return () => sse?.disconnect();
	});

	$effect(() => {
		if (conversationStore.pinnedToBottom && timelineEl) {
			timelineEl.scrollTop = timelineEl.scrollHeight;
		}
	});
</script>

<div class="space-y-4">
	<div class="flex items-center gap-3">
		<a href="/conversations/{page.params.id}" class="text-muted-foreground hover:text-foreground text-sm">&larr; Timeline</a>
		<h1 class="text-xl font-semibold font-mono">{truncateId(page.params.id)}</h1>
		{#if conversationStore.conversation}
			<StatusBadge status={conversationStore.conversation.status} />
		{/if}
		<span class="text-sm text-blue-400">Live</span>
	</div>

	<!-- Peer status row -->
	<div class="grid grid-cols-2 gap-3 md:grid-cols-4 lg:grid-cols-6">
		{#each [...conversationStore.peers.values()] as peer (peer.id)}
			<PeerStatusCard id={peer.id} state={peer.state} turnCount={peer.turnCount} />
		{/each}
	</div>

	<!-- Metrics -->
	<MetricsBar />

	<!-- Compact timeline -->
	<div bind:this={timelineEl} class="max-h-[calc(100vh-22rem)] space-y-2 overflow-auto">
		{#each conversationStore.turns as turn (turn.index)}
			<TurnCard {turn} />
		{/each}
	</div>
</div>
```

- [ ] **Step 4: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/PeerStatusCard.svelte src/lib/components/MetricsBar.svelte src/routes/conversations/\[id\]/live/+page.svelte
git commit -m "feat: add Live Conversation Monitor view with peer status and metrics"
```

---

### Task 13: Peer Topology Canvas (View 4)

**Files:**
- Create: `src/lib/components/TopologyCanvas.svelte`
- Create: `src/routes/conversations/[id]/topology/+page.svelte`

- [ ] **Step 1: Create TopologyCanvas component**

```svelte
<!-- src/lib/components/TopologyCanvas.svelte -->
<script lang="ts">
	import { SvelteFlow, Background, Controls, MiniMap, type Node, type Edge } from '@xyflow/svelte';
	import '@xyflow/svelte/dist/style.css';
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import { peerColor } from '$lib/utils/peer-colors';

	let nodes = $state.raw<Node[]>([]);
	let edges = $state.raw<Edge[]>([]);

	$effect(() => {
		const peerList = [...conversationStore.peers.values()];
		const count = peerList.length;

		nodes = peerList.map((peer, i) => {
			const angle = (2 * Math.PI * i) / count;
			const radius = count <= 2 ? 0 : 200;
			const x = count <= 2 ? i * 350 : 300 + radius * Math.cos(angle);
			const y = count <= 2 ? 100 : 300 + radius * Math.sin(angle);

			return {
				id: peer.id,
				position: { x, y },
				data: {
					label: peer.id,
					state: peer.state,
					turnCount: peer.turnCount,
				},
				style: `border: 2px solid ${peerColor(peer.id)}; border-radius: 8px; padding: 12px; background: var(--card);`,
			};
		});

		// Create edges from turn history
		const edgeSet = new Set<string>();
		for (const turn of conversationStore.turns) {
			const from = turn.from_id ?? turn.from;
			const to = turn.to_id;
			if (from && to) {
				const key = `${from}-${to}`;
				if (!edgeSet.has(key)) {
					edgeSet.add(key);
					edges = [
						...edges.filter((e) => e.id !== `e-${key}`),
						{
							id: `e-${key}`,
							source: from,
							target: to,
							animated: true,
							style: `stroke: ${peerColor(from)};`,
						},
					];
				}
			}
		}
	});
</script>

<div class="h-full w-full" style="min-height: 500px;">
	<SvelteFlow bind:nodes bind:edges fitView>
		<Background />
		<Controls />
		<MiniMap />
	</SvelteFlow>
</div>
```

- [ ] **Step 2: Create topology page**

```svelte
<!-- src/routes/conversations/[id]/topology/+page.svelte -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { getConversation, getConversationTurns } from '$lib/api/rest';
	import { conversationStore } from '$lib/stores/conversation.svelte';
	import { createConversationStream } from '$lib/api/sse';
	import TopologyCanvas from '$lib/components/TopologyCanvas.svelte';
	import StatusBadge from '$lib/components/StatusBadge.svelte';
	import { truncateId } from '$lib/utils/format';

	let sse: ReturnType<typeof createConversationStream> | null = null;

	onMount(async () => {
		const [conv, turnsResp] = await Promise.all([
			getConversation(page.params.id),
			getConversationTurns(page.params.id, 100),
		]);
		conversationStore.setConversation(conv);
		conversationStore.setTurns(turnsResp.data);

		if (conv.status === 'running') {
			sse = createConversationStream(
				page.params.id,
				(event) => {
					if (event.type === 'turn') conversationStore.addTurn(event.data);
					if (event.type === 'envelope') conversationStore.handleEnvelope(event.data);
					if (event.type === 'control') conversationStore.handleControl(event.data);
				},
				() => {},
			);
		}

		return () => sse?.disconnect();
	});
</script>

<div class="flex h-full flex-col space-y-4">
	<div class="flex items-center gap-3">
		<a href="/conversations/{page.params.id}" class="text-muted-foreground hover:text-foreground text-sm">&larr; Timeline</a>
		<h1 class="text-xl font-semibold font-mono">{truncateId(page.params.id)}</h1>
		{#if conversationStore.conversation}
			<StatusBadge status={conversationStore.conversation.status} />
		{/if}
		<span class="text-sm text-emerald-400">Topology</span>
	</div>

	<div class="flex-1">
		<TopologyCanvas />
	</div>
</div>
```

- [ ] **Step 3: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/TopologyCanvas.svelte src/routes/conversations/\[id\]/topology/+page.svelte
git commit -m "feat: add Peer Topology Canvas view with Svelte Flow"
```

---

### Task 14: Verdict Overlay (View 5)

**Files:**
- Create: `src/lib/components/VerdictOverlay.svelte`

- [ ] **Step 1: Create VerdictOverlay**

```svelte
<!-- src/lib/components/VerdictOverlay.svelte -->
<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import { Button } from '$lib/components/ui/button';
	import StatusBadge from './StatusBadge.svelte';
	import type { Verdict } from '$lib/api/types';

	let { verdict, ondismiss }: { verdict: Verdict; ondismiss: () => void } = $props();

	const decisionColors: Record<string, string> = {
		complete: 'text-green-400',
		continue: 'text-blue-400',
		timeout: 'text-yellow-400',
		error: 'text-red-400',
	};
</script>

<Dialog.Root open={true} onOpenChange={(open) => { if (!open) ondismiss(); }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title class="flex items-center gap-2">
				Conversation Verdict
				<StatusBadge status={verdict.status} />
			</Dialog.Title>
		</Dialog.Header>

		<div class="space-y-4">
			<div>
				<span class="text-sm text-muted-foreground">Decision: </span>
				<span class="text-lg font-semibold {decisionColors[verdict.decision] ?? ''}">
					{verdict.decision}
				</span>
			</div>

			<div>
				<span class="text-sm text-muted-foreground">Reason: </span>
				<p class="text-sm">{verdict.reason}</p>
			</div>

			<div>
				<span class="text-sm text-muted-foreground">Final Status: </span>
				<StatusBadge status={verdict.status} />
			</div>
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={ondismiss}>Dismiss</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
```

- [ ] **Step 2: Commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add src/lib/components/VerdictOverlay.svelte
git commit -m "feat: add Verdict/Termination Overlay component"
```

---

### Task 15: Build Verification + Final Commit

- [ ] **Step 1: Verify TypeScript compiles**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npx tsc --noEmit
```

Expected: no errors

- [ ] **Step 2: Verify build succeeds**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npm run build
```

Expected: build succeeds

- [ ] **Step 3: Verify dev server runs**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
npm run dev -- --port 5173
```

Expected: dev server starts, all pages load (they will show empty state since no API is running)

- [ ] **Step 4: Final commit**

```bash
cd /home/kamil-rybacki/Code/vitis-ui
git add -A
git commit -m "chore: verify build and dev server"
```
