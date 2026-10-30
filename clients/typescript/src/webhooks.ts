import { InvalidSignatureError } from "./errors.js";
import type { WebhookPayload } from "./types.js";

/** The header carrying the HMAC-SHA256 of a delivery's body. */
export const SIGNATURE_HEADER = "X-Signature-256";

const encoder = new TextEncoder();

function toBytes(body: string | Uint8Array): Uint8Array {
  return typeof body === "string" ? encoder.encode(body) : body;
}

async function hmacKey(secret: string, usage: "sign" | "verify"): Promise<CryptoKey> {
  return crypto.subtle.importKey("raw", encoder.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, [usage]);
}

function toHex(bytes: ArrayBuffer): string {
  return Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, "0")).join("");
}

function fromHex(hex: string): Uint8Array | null {
  if (hex.length === 0 || hex.length % 2 !== 0 || !/^[0-9a-fA-F]+$/.test(hex)) return null;
  const out = new Uint8Array(hex.length / 2);
  for (let i = 0; i < out.length; i++) out[i] = parseInt(hex.slice(i * 2, i * 2 + 2), 16);
  return out;
}

/**
 * Produces the signature header value Quorum would send for `body`. For
 * tests and for proxies that re-sign deliveries.
 */
export async function sign(body: string | Uint8Array, secret: string): Promise<string> {
  const key = await hmacKey(secret, "sign");
  const mac = await crypto.subtle.sign("HMAC", key, toBytes(body) as BufferSource);
  return `sha256=${toHex(mac)}`;
}

/**
 * Reports whether `signature`, in the form `sha256=<hex>`, is the HMAC-SHA256
 * of `body` under `secret`. The comparison is constant-time.
 */
export async function verifySignature(
  body: string | Uint8Array,
  signature: string | null | undefined,
  secret: string,
): Promise<boolean> {
  if (!signature || !secret || !signature.startsWith("sha256=")) return false;
  const given = fromHex(signature.slice("sha256=".length));
  if (!given) return false;
  const key = await hmacKey(secret, "verify");
  return crypto.subtle.verify("HMAC", key, given as BufferSource, toBytes(body) as BufferSource);
}

/**
 * Verifies a delivery's signature against `secret` and decodes it. Throws
 * `InvalidSignatureError` for anything not signed with the secret; reject the
 * request when it does. Respond 2xx once you have durably recorded the event,
 * otherwise Quorum retries.
 *
 * Pass the raw body exactly as received: re-serialised JSON will not verify.
 */
export async function parseWebhook(
  body: string | Uint8Array,
  signature: string | null | undefined,
  secret: string,
): Promise<WebhookPayload> {
  if (!(await verifySignature(body, signature, secret))) throw new InvalidSignatureError();
  const text = typeof body === "string" ? body : new TextDecoder().decode(body);
  return JSON.parse(text) as WebhookPayload;
}
