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

import requests as http_client
from flask import Flask, jsonify, redirect, render_template, request, url_for

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

QUORUM_API_URL = os.environ.get("QUORUM_API_URL", "http://localhost:8080")
SELF_URL = os.environ.get("SELF_URL", "http://localhost:3003")
WEBHOOK_SECRET = os.environ.get("WEBHOOK_SECRET", "access-webhook-secret")
PORT = int(os.environ.get("PORT", "3003"))

# ---------------------------------------------------------------------------
# App setup
# ---------------------------------------------------------------------------

app = Flask(__name__)
logging.basicConfig(level=logging.INFO, format="%(asctime)s [%(levelname)s] %(message)s")
log = logging.getLogger("access-portal")

# ---------------------------------------------------------------------------
# In-memory access request store
# ---------------------------------------------------------------------------

access_requests: dict[str, dict] = {}


def new_access_request(
    requester: str,
    system_name: str,
    access_level: str,
    justification: str,
) -> dict:
    req_id = str(uuid.uuid4())
    now = datetime.now(timezone.utc).isoformat()
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


# ---------------------------------------------------------------------------
# Quorum API helpers
# ---------------------------------------------------------------------------

QUORUM_HEADERS = {
    "Content-Type": "application/json",
    "X-User-ID": "access-portal-service",
    "X-User-Roles": "service",
    "X-Tenant-ID": "access-portal",
}


def register_policy() -> None:
    """Register the access_request policy with Quorum on startup (idempotent)."""
    policy = {
        "name": "System Access Request",
        "request_type": "access_request",
        "stages": [
            {
                "index": 0,
                "name": "Security Review",
                "required_approvals": 2,
                "max_checkers": 3,
                "rejection_policy": "threshold",
                "allowed_checker_roles": ["security"],
            }
        ],
        "auto_expire_duration": "72h",
    }
    try:
        resp = http_client.post(
            f"{QUORUM_API_URL}/api/v1/policies",
            json=policy,
            headers=QUORUM_HEADERS,
            timeout=10,
        )
        if resp.status_code == 201:
            log.info("Registered access_request policy with Quorum")
        elif resp.status_code == 409:
            log.info("access_request policy already exists in Quorum (409 conflict)")
        else:
            log.warning("Unexpected response registering policy: %s %s", resp.status_code, resp.text)
    except Exception as exc:
        log.error("Failed to register policy with Quorum: %s", exc)


def submit_to_quorum(access_req: dict, eligible_reviewers: list[str]) -> str | None:
    """Submit an access request to Quorum and return the Quorum request ID."""
    callback_url = f"{SELF_URL}/webhooks/quorum"
    body = {
        "type": "access_request",
        "payload": {
            "local_id": access_req["id"],
            "requester": access_req["requester"],
            "system_name": access_req["system_name"],
            "access_level": access_req["access_level"],
            "justification": access_req["justification"],
        },
        "callback_url": callback_url,
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
    sorted_requests = sorted(
        access_requests.values(),
        key=lambda r: r["created_at"],
        reverse=True,
    )
    return render_template("dashboard.html", requests=sorted_requests)


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

    access_req = new_access_request(requester, system_name, access_level, justification)
    access_requests[access_req["id"]] = access_req

    quorum_id = submit_to_quorum(access_req, eligible_reviewers)
    if quorum_id:
        access_req["quorum_request_id"] = quorum_id
    else:
        access_req["status"] = "error"

    return redirect(url_for("request_detail", request_id=access_req["id"]))


@app.route("/request/<request_id>")
def request_detail(request_id: str):
    """Request detail page with approval progress."""
    access_req = access_requests.get(request_id)
    if not access_req:
        return render_template("layout.html", error="Request not found"), 404

    quorum_data = None
    if access_req.get("quorum_request_id"):
        quorum_data = fetch_quorum_request(access_req["quorum_request_id"])

    return render_template("detail.html", access_req=access_req, quorum_data=quorum_data)


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

    # Find the local access request by Quorum request ID
    local_req = None
    for req in access_requests.values():
        if req.get("quorum_request_id") == quorum_id:
            local_req = req
            break

    if not local_req:
        log.warning("No local request found for Quorum ID: %s", quorum_id)
        return jsonify({"status": "ignored"}), 200

    # Map Quorum events to local status
    if event == "approved":
        local_req["status"] = "granted"
        log.info("Access GRANTED for request %s (%s)", local_req["id"], local_req["system_name"])
    elif event == "rejected":
        local_req["status"] = "denied"
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
    register_policy()
    app.run(host="0.0.0.0", port=PORT, debug=True)
