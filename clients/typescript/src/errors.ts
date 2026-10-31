/** An error response from the Quorum server. */
export class QuorumError extends Error {
  override readonly name = "QuorumError";

  constructor(
    /** HTTP status code. */
    readonly status: number,
    /** The server's error message. */
    message: string,
    /** Method and path of the failed call. */
    readonly request: { method: string; path: string },
  ) {
    super(`quorum: ${request.method} ${request.path}: ${status} ${message}`);
    /** The message exactly as the server sent it. */
    this.detail = message;
  }

  readonly detail: string;

  get isBadRequest(): boolean {
    return this.status === 400;
  }
  get isUnauthorized(): boolean {
    return this.status === 401;
  }
  get isForbidden(): boolean {
    return this.status === 403;
  }
  get isNotFound(): boolean {
    return this.status === 404;
  }
  get isConflict(): boolean {
    return this.status === 409;
  }
}

/** A webhook delivery whose signature did not verify. */
export class InvalidSignatureError extends Error {
  override readonly name = "InvalidSignatureError";
  constructor() {
    super("quorum: webhook signature invalid");
  }
}

export function isQuorumError(err: unknown): err is QuorumError {
  return err instanceof QuorumError;
}
export function isNotFound(err: unknown): boolean {
  return isQuorumError(err) && err.isNotFound;
}
export function isConflict(err: unknown): boolean {
  return isQuorumError(err) && err.isConflict;
}
export function isForbidden(err: unknown): boolean {
  return isQuorumError(err) && err.isForbidden;
}
export function isUnauthorized(err: unknown): boolean {
  return isQuorumError(err) && err.isUnauthorized;
}
export function isBadRequest(err: unknown): boolean {
  return isQuorumError(err) && err.isBadRequest;
}
