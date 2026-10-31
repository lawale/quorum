import type { RequestStatus } from "./types.js";
import { isTerminal } from "./types.js";

/** One message from a request's event stream. */
export interface StreamEvent {
  requestId: string;
  /**
   * Set only on the closing event of an already-terminal request. For an
   * "updated" event it is absent: fetch the request to see the change.
   */
  status?: RequestStatus;
}

/** Whether the stream has nothing more to say after this event. */
export function isTerminalEvent(ev: StreamEvent): boolean {
  return ev.status !== undefined && isTerminal(ev.status);
}

/** Parses a text/event-stream body into Quorum events. */
export async function* readEvents(body: ReadableStream<Uint8Array>): AsyncGenerator<StreamEvent> {
  const reader = body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let name = "";
  let data = "";
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let nl: number;
      while ((nl = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, nl).replace(/\r$/, "");
        buffer = buffer.slice(nl + 1);
        if (line === "") {
          if (name !== "" || data !== "") {
            yield parseEvent(name, data);
            name = "";
            data = "";
          }
        } else if (line.startsWith(":")) {
          // keepalive comment
        } else if (line.startsWith("event:")) {
          name = line.slice("event:".length).trim();
        } else if (line.startsWith("data:")) {
          data += line.slice("data:".length).trim();
        }
      }
    }
  } finally {
    reader.releaseLock();
  }
}

function parseEvent(name: string, data: string): StreamEvent {
  let parsed: { request_id?: unknown; status?: unknown };
  try {
    parsed = JSON.parse(data) as typeof parsed;
  } catch (err) {
    throw new Error(`quorum: malformed "${name}" event: ${(err as Error).message}`);
  }
  const ev: StreamEvent = { requestId: String(parsed.request_id ?? "") };
  if (typeof parsed.status === "string") ev.status = parsed.status as RequestStatus;
  return ev;
}
