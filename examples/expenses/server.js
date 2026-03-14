const express = require("express");
const crypto = require("crypto");
const path = require("path");

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------
const QUORUM_API_URL = process.env.QUORUM_API_URL || "http://localhost:8080";
const SELF_URL = process.env.SELF_URL || "http://localhost:3002";
const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || "expenses-webhook-secret";
const PORT = parseInt(process.env.PORT, 10) || 3002;

// ---------------------------------------------------------------------------
// In-memory expense store
// ---------------------------------------------------------------------------
const expenses = new Map();

function createExpense({ employeeName, amount, category, description }) {
  const id = crypto.randomUUID();
  const expense = {
    id,
    employeeName,
    amount: parseFloat(amount),
    category,
    description,
    status: "pending",
    quorumRequestId: null,
    approvals: [],
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };
  expenses.set(id, expense);
  return expense;
}

// ---------------------------------------------------------------------------
// Quorum API helpers
// ---------------------------------------------------------------------------
async function quorumFetch(urlPath, options = {}) {
  const url = `${QUORUM_API_URL}${urlPath}`;
  const res = await fetch(url, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });
  const body = await res.text();
  let json;
  try {
    json = JSON.parse(body);
  } catch {
    json = null;
  }
  return { status: res.status, data: json, raw: body };
}

async function registerPolicy() {
  const policy = {
    name: "Expense Approval",
    request_type: "expense_approval",
    stages: [
      {
        index: 0,
        name: "Manager Approval",
        required_approvals: 1,
        allowed_checker_roles: ["manager", "admin"],
        rejection_policy: "any",
      },
    ],
  };

  const res = await quorumFetch("/api/v1/policies", {
    method: "POST",
    body: JSON.stringify(policy),
  });

  if (res.status === 201) {
    console.log("[startup] Registered expense_approval policy with Quorum");
  } else if (res.status === 409) {
    console.log("[startup] expense_approval policy already exists (409 conflict) - OK");
  } else {
    console.error("[startup] Failed to register policy:", res.status, res.raw);
  }
}

async function submitRequest(expense, userId) {
  const payload = {
    type: "expense_approval",
    payload: {
      expense_id: expense.id,
      employee_name: expense.employeeName,
      amount: expense.amount,
      category: expense.category,
      description: expense.description,
    },
    callback_url: `${SELF_URL}/webhooks/quorum`,
  };

  const res = await quorumFetch("/api/v1/requests", {
    method: "POST",
    body: JSON.stringify(payload),
    headers: {
      "X-User-ID": userId,
      "X-User-Roles": "employee",
    },
  });

  return res;
}

async function getRequest(requestId) {
  const res = await quorumFetch(`/api/v1/requests/${requestId}`, {
    method: "GET",
    headers: {
      "X-User-ID": "system",
      "X-User-Roles": "admin",
    },
  });
  return res;
}

// ---------------------------------------------------------------------------
// HMAC-SHA256 verification
// ---------------------------------------------------------------------------
function verifySignature(rawBody, signature) {
  if (!signature) return false;
  const expected =
    "sha256=" +
    crypto.createHmac("sha256", WEBHOOK_SECRET).update(rawBody).digest("hex");
  return crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(signature));
}

// ---------------------------------------------------------------------------
// Express app
// ---------------------------------------------------------------------------
const app = express();

// Parse JSON with raw body capture for webhook signature verification
app.use(
  express.json({
    verify: (req, _res, buf) => {
      req.rawBody = buf;
    },
  })
);
app.use(express.urlencoded({ extended: true }));

// View engine
app.set("view engine", "ejs");
app.set("views", path.join(__dirname, "views"));

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

// Dashboard - list all expenses
app.get("/", (_req, res) => {
  const allExpenses = Array.from(expenses.values()).sort(
    (a, b) => new Date(b.createdAt) - new Date(a.createdAt)
  );
  res.render("dashboard", { expenses: allExpenses });
});

// New expense form
app.get("/expense/new", (_req, res) => {
  res.render("new_expense", { error: null });
});

// Create expense and submit to Quorum
app.post("/expense/new", async (req, res) => {
  const { employeeName, amount, category, description } = req.body;

  // Basic validation
  if (!employeeName || !amount || !category) {
    return res.render("new_expense", {
      error: "Employee name, amount, and category are required.",
    });
  }

  const parsedAmount = parseFloat(amount);
  if (isNaN(parsedAmount) || parsedAmount <= 0) {
    return res.render("new_expense", {
      error: "Amount must be a positive number.",
    });
  }

  const expense = createExpense({ employeeName, amount, category, description });

  // Submit to Quorum for approval
  try {
    const result = await submitRequest(expense, employeeName.toLowerCase().replace(/\s+/g, "."));
    if (result.status === 201 && result.data) {
      expense.quorumRequestId = result.data.id;
      expense.updatedAt = new Date().toISOString();
      console.log(`[expense] Created expense ${expense.id}, Quorum request: ${result.data.id}`);
    } else {
      console.error(`[expense] Failed to submit to Quorum: ${result.status}`, result.raw);
      expense.status = "error";
      expense.updatedAt = new Date().toISOString();
    }
  } catch (err) {
    console.error("[expense] Error submitting to Quorum:", err.message);
    expense.status = "error";
    expense.updatedAt = new Date().toISOString();
  }

  res.redirect(`/expense/${expense.id}`);
});

// Expense detail - polls Quorum for latest status
app.get("/expense/:id", async (req, res) => {
  const expense = expenses.get(req.params.id);
  if (!expense) {
    return res.status(404).send("Expense not found");
  }

  let quorumRequest = null;

  // Poll Quorum for latest status if we have a request ID
  if (expense.quorumRequestId) {
    try {
      const result = await getRequest(expense.quorumRequestId);
      if (result.status === 200 && result.data) {
        quorumRequest = result.data;
        // Sync status from Quorum
        if (result.data.status !== expense.status) {
          expense.status = result.data.status;
          expense.updatedAt = new Date().toISOString();
        }
        if (result.data.approvals) {
          expense.approvals = result.data.approvals;
        }
      }
    } catch (err) {
      console.error("[expense] Error polling Quorum:", err.message);
    }
  }

  res.render("detail", { expense, quorumRequest });
});

// ---------------------------------------------------------------------------
// Webhook endpoint - receives callbacks from Quorum
// ---------------------------------------------------------------------------
app.post("/webhooks/quorum", (req, res) => {
  const signature = req.headers["x-signature-256"];

  // Verify HMAC-SHA256 signature
  if (WEBHOOK_SECRET && req.rawBody) {
    if (!verifySignature(req.rawBody, signature)) {
      console.warn("[webhook] Invalid signature, rejecting");
      return res.status(401).json({ error: "invalid signature" });
    }
  }

  const { event, request: quorumReq, approvals, timestamp } = req.body;
  console.log(`[webhook] Received event=${event} request=${quorumReq?.id} at ${timestamp}`);

  if (!quorumReq || !quorumReq.id) {
    return res.status(400).json({ error: "missing request data" });
  }

  // Find local expense by Quorum request ID
  let matchedExpense = null;
  for (const expense of expenses.values()) {
    if (expense.quorumRequestId === quorumReq.id) {
      matchedExpense = expense;
      break;
    }
  }

  if (!matchedExpense) {
    // Could also match via payload.expense_id if available
    const payloadExpenseId = quorumReq.payload?.expense_id;
    if (payloadExpenseId) {
      matchedExpense = expenses.get(payloadExpenseId);
    }
  }

  if (matchedExpense) {
    matchedExpense.status = event; // "approved" | "rejected"
    matchedExpense.approvals = approvals || [];
    matchedExpense.updatedAt = new Date().toISOString();
    console.log(`[webhook] Updated expense ${matchedExpense.id} -> ${event}`);
  } else {
    console.warn(`[webhook] No matching expense found for Quorum request ${quorumReq.id}`);
  }

  res.status(200).json({ received: true });
});

// ---------------------------------------------------------------------------
// Startup
// ---------------------------------------------------------------------------
app.listen(PORT, async () => {
  console.log(`Expense Tracker running at http://localhost:${PORT}`);
  console.log(`Quorum API: ${QUORUM_API_URL}`);
  console.log(`Webhook callback: ${SELF_URL}/webhooks/quorum`);
  console.log();

  // Register policy on startup (idempotent)
  try {
    await registerPolicy();
  } catch (err) {
    console.error("[startup] Could not reach Quorum API:", err.message);
    console.error("[startup] Make sure Quorum is running. The app will still work for viewing.");
  }
});
