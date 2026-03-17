"""
Access Request Portal - Example consumer app for Quorum maker-checker system.

Demonstrates system access requests where security teams approve/reject
using threshold-based voting (2 of 3 security reviewers must approve).
"""

import hashlib
import hmac
import json
import logging
import os
import uuid
from datetime import datetime, timezone

import psycopg2
import psycopg2.extras
import requests as http_client
from flask import Flask, jsonify, redirect, render_template, request, url_for

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

QUORUM_API_URL = os.environ.get("QUORUM_API_URL", "http://localhost:8080")
QUORUM_PUBLIC_URL = os.environ.get("QUORUM_PUBLIC_URL", QUORUM_API_URL)
SELF_URL = os.environ.get("SELF_URL", "http://localhost:3003")
WEBHOOK_SECRET = os.environ.get("WEBHOOK_SECRET", "access-webhook-secret")
DATABASE_URL = os.environ.get("DATABASE_URL", "postgres://quorum:quorum@localhost:5432/quorum")
PORT = int(os.environ.get("PORT", "3003"))

# ---------------------------------------------------------------------------
# App setup
# ---------------------------------------------------------------------------

app = Flask(__name__)
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger("access-portal")

# ---------------------------------------------------------------------------
# PostgreSQL access request store
# ---------------------------------------------------------------------------


def get_db():
    conn = psycopg2.connect(DATABASE_URL)
    conn.autocommit = True
    return conn


def init_db() -> None:
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute("""
                CREATE TABLE IF NOT EXISTS access_requests (
                    id                  TEXT PRIMARY KEY,
                    requester           TEXT NOT NULL,
                    system_name         TEXT NOT NULL,
                    access_level        TEXT NOT NULL,
                    justification       TEXT NOT NULL,
                    status              TEXT NOT NULL DEFAULT 'pending',
                    quorum_request_id   TEXT,
                    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
                );
            """)
        log.info("Database schema and tables initialized")
    finally:
        conn.close()


def db_create_request(
    requester: str,
    system_name: str,
    access_level: str,
    justification: str,
) -> dict:
    req_id = str(uuid.uuid4())
    now = datetime.now(timezone.utc).isoformat()
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute(
                """INSERT INTO access_requests
                   (id, requester, system_name, access_level, justification, status, created_at)
                   VALUES (%s, %s, %s, %s, %s, 'pending', %s)""",
                (req_id, requester, system_name, access_level, justification, now),
            )
    finally:
        conn.close()

    return {
        "id": req_id,
        "requester": requester,
        "system_name": system_name,
        "access_level": access_level,
        "justification": justification,
        "status": "pending",
        "quorum_request_id": None,
        "created_at": now,
    }


def db_get_by_id(req_id: str) -> dict | None:
    conn = get_db()
    try:
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                "SELECT * FROM access_requests WHERE id = %s", (req_id,)
            )
            row = cur.fetchone()
            return _row_to_dict(row) if row else None
    finally:
        conn.close()


def db_get_by_quorum_id(quorum_id: str) -> dict | None:
    conn = get_db()
    try:
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                "SELECT * FROM access_requests WHERE quorum_request_id = %s",
                (quorum_id,),
            )
            row = cur.fetchone()
            return _row_to_dict(row) if row else None
    finally:
        conn.close()


def db_list_requests() -> list[dict]:
    conn = get_db()
    try:
        with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
            cur.execute(
                "SELECT * FROM access_requests ORDER BY created_at DESC"
            )
            return [_row_to_dict(row) for row in cur.fetchall()]
    finally:
        conn.close()


def db_update_status(req_id: str, status: str) -> None:
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE access_requests SET status = %s WHERE id = %s",
                (status, req_id),
            )
    finally:
        conn.close()


def db_set_quorum_id(req_id: str, quorum_request_id: str) -> None:
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute(
                "UPDATE access_requests SET quorum_request_id = %s WHERE id = %s",
                (quorum_request_id, req_id),
            )
    finally:
        conn.close()


def _row_to_dict(row: dict) -> dict:
    created_at = row["created_at"]
    if hasattr(created_at, "isoformat"):
        created_at = created_at.isoformat()
    return {
        "id": row["id"],
        "requester": row["requester"],
        "system_name": row["system_name"],
        "access_level": row["access_level"],
        "justification": row["justification"],
        "status": row["status"],
        "quorum_request_id": row["quorum_request_id"],
        "created_at": created_at,
    }


# ---------------------------------------------------------------------------
# Quorum API helpers
# ---------------------------------------------------------------------------

QUORUM_HEADERS = {
    "Content-Type": "application/json",
    "X-User-ID": "access-portal-service",
    "X-User-Roles": "service",
    "X-Tenant-ID": "access-portal",
}


def submit_to_quorum(access_req: dict, eligible_reviewers: list[str]) -> str | None:
    """Submit an access request to Quorum and return the Quorum request ID."""
    body = {
        "type": "access_request",
        "payload": {
            "local_id": access_req["id"],
            "requester": access_req["requester"],
            "system_name": access_req["system_name"],
            "access_level": access_req["access_level"],
            "justification": access_req["justification"],
        },
        "eligible_reviewers": eligible_reviewers,
    }
    headers = {
        **QUORUM_HEADERS,
        "X-User-ID": access_req["requester"],
    }
    try:
        resp = http_client.post(
            f"{QUORUM_API_URL}/api/v1/requests",
            json=body,
            headers=headers,
            timeout=10,
        )
        if resp.status_code == 201:
            data = resp.json()
            quorum_id = data.get("id")
            log.info("Submitted request to Quorum: %s -> %s", access_req["id"], quorum_id)
            return quorum_id
        else:
            log.error("Quorum rejected request: %s %s", resp.status_code, resp.text)
            return None
    except Exception as exc:
        log.error("Failed to submit request to Quorum: %s", exc)
        return None


def fetch_quorum_request(quorum_request_id: str) -> dict | None:
    """Fetch the current state of a request from Quorum."""
    try:
        resp = http_client.get(
            f"{QUORUM_API_URL}/api/v1/requests/{quorum_request_id}",
            headers=QUORUM_HEADERS,
            timeout=10,
        )
        if resp.status_code == 200:
            return resp.json()
        return None
    except Exception:
        return None


# ---------------------------------------------------------------------------
# HMAC signature verification
# ---------------------------------------------------------------------------


def verify_signature(raw_body: bytes, signature_header: str | None) -> bool:
    """Verify HMAC-SHA256 signature from Quorum webhook delivery."""
    if not signature_header:
        return False
    if not signature_header.startswith("sha256="):
        return False
    expected_sig = signature_header[len("sha256="):]
    computed = hmac.new(
        WEBHOOK_SECRET.encode(),
        raw_body,
        hashlib.sha256,
    ).hexdigest()
    return hmac.compare_digest(computed, expected_sig)


# ---------------------------------------------------------------------------
# Routes
# ---------------------------------------------------------------------------


@app.route("/")
def dashboard():
    """Dashboard listing all access requests with status badges."""
    sorted_requests = db_list_requests()
    return render_template("dashboard.html", requests=sorted_requests, quorum_url=QUORUM_PUBLIC_URL)


@app.route("/request/new", methods=["GET"])
def new_request_form():
    """Form to request system access."""
    return render_template("new_request.html")


@app.route("/request/new", methods=["POST"])
def create_request():
    """Create an access request locally, then submit to Quorum."""
    requester = request.form.get("requester", "").strip()
    system_name = request.form.get("system_name", "").strip()
    access_level = request.form.get("access_level", "").strip()
    justification = request.form.get("justification", "").strip()
    reviewers_raw = request.form.get("eligible_reviewers", "").strip()

    if not all([requester, system_name, access_level, justification]):
        return render_template(
            "new_request.html",
            error="All fields are required.",
            form=request.form,
        ), 400

    eligible_reviewers = [r.strip() for r in reviewers_raw.split(",") if r.strip()]
    if not eligible_reviewers:
        eligible_reviewers = ["security-alice", "security-bob", "security-charlie"]

    access_req = db_create_request(requester, system_name, access_level, justification)

    quorum_id = submit_to_quorum(access_req, eligible_reviewers)
    if quorum_id:
        db_set_quorum_id(access_req["id"], quorum_id)
        access_req["quorum_request_id"] = quorum_id
    else:
        db_update_status(access_req["id"], "error")

    return redirect(url_for("request_detail", request_id=access_req["id"]))


@app.route("/api/requests", methods=["POST"])
def create_request_api():
    """JSON API endpoint used by the seed script."""
    data = request.get_json()
    if not data:
        return jsonify({"error": "invalid JSON"}), 400

    requester = (data.get("requester") or "").strip()
    system_name = (data.get("system_name") or "").strip()
    access_level = (data.get("access_level") or "").strip()
    justification = (data.get("justification") or "").strip()
    eligible_reviewers = data.get("eligible_reviewers") or [
        "security-alice", "security-bob", "security-charlie"
    ]

    if not all([requester, system_name, access_level, justification]):
        return jsonify({"error": "requester, system_name, access_level, and justification are required"}), 400

    access_req = db_create_request(requester, system_name, access_level, justification)

    quorum_id = submit_to_quorum(access_req, eligible_reviewers)
    if quorum_id:
        db_set_quorum_id(access_req["id"], quorum_id)
        access_req["quorum_request_id"] = quorum_id
    else:
        db_update_status(access_req["id"], "error")

    return jsonify(access_req), 201


@app.route("/request/<request_id>")
def request_detail(request_id: str):
    """Request detail page with approval progress."""
    access_req = db_get_by_id(request_id)
    if not access_req:
        return render_template("layout.html", error="Request not found"), 404

    quorum_data = None
    if access_req.get("quorum_request_id"):
        quorum_data = fetch_quorum_request(access_req["quorum_request_id"])

    return render_template("detail.html", access_req=access_req, quorum_data=quorum_data, quorum_url=QUORUM_PUBLIC_URL)


@app.route("/webhooks/quorum", methods=["POST"])
def webhook_callback():
    """Webhook callback from Quorum. Verifies HMAC-SHA256 signature and
    updates local request status (granted/denied)."""
    raw_body = request.get_data()

    signature = request.headers.get("X-Signature-256")
    if not verify_signature(raw_body, signature):
        log.warning("Webhook signature verification failed")
        return jsonify({"error": "invalid signature"}), 401

    try:
        payload = json.loads(raw_body)
    except json.JSONDecodeError:
        return jsonify({"error": "invalid JSON"}), 400

    event = payload.get("event", "")
    quorum_request = payload.get("request", {})
    quorum_id = quorum_request.get("id")

    log.info("Received webhook: event=%s quorum_id=%s", event, quorum_id)

    local_req = db_get_by_quorum_id(quorum_id) if quorum_id else None

    if not local_req:
        log.warning("No local request found for Quorum ID: %s", quorum_id)
        return jsonify({"status": "ignored"}), 200

    if event == "approved":
        db_update_status(local_req["id"], "granted")
        log.info("Access GRANTED for request %s (%s)", local_req["id"], local_req["system_name"])
    elif event == "rejected":
        db_update_status(local_req["id"], "denied")
        log.info("Access DENIED for request %s (%s)", local_req["id"], local_req["system_name"])
    else:
        log.info("Unhandled webhook event: %s", event)

    return jsonify({"status": "ok"}), 200


# ---------------------------------------------------------------------------
# Startup
# ---------------------------------------------------------------------------

if __name__ == "__main__":
    log.info("Starting Access Request Portal on port %d", PORT)
    log.info("Quorum API: %s", QUORUM_API_URL)
    log.info("Self URL: %s", SELF_URL)
    init_db()
    app.run(host="0.0.0.0", port=PORT, debug=True)
