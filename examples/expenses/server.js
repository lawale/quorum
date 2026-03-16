const express = require("express");
const crypto = require("crypto");
const path = require("path");
const { Pool } = require("pg");

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------
const QUORUM_API_URL = process.env.QUORUM_API_URL || "http://localhost:8080";
const QUORUM_PUBLIC_URL = process.env.QUORUM_PUBLIC_URL || QUORUM_API_URL;
const SELF_URL = process.env.SELF_URL || "http://localhost:3002";
const WEBHOOK_SECRET = process.env.WEBHOOK_SECRET || "expenses-webhook-secret";
const DATABASE_URL = process.env.DATABASE_URL || "postgres://quorum:quorum@localhost:5432/quorum";
const PORT = parseInt(process.env.PORT, 10) || 3002;

// ---------------------------------------------------------------------------
// PostgreSQL expense store
// ---------------------------------------------------------------------------
const pool = new Pool({ connectionString: DATABASE_URL });

async function initDB() {
  const ddl = `
    CREATE TABLE IF NOT EXISTS expenses (
      id                TEXT PRIMARY KEY,
      employee_name     TEXT NOT NULL,
      amount            NUMERIC(12,2) NOT NULL,
      category          TEXT NOT NULL,
      description       TEXT,
      status            TEXT NOT NULL DEFAULT 'pending',
      quorum_request_id TEXT,
      approvals         JSONB NOT NULL DEFAULT '[]',
      created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );
  `;
  await pool.query(ddl);
  console.log("[db] Schema and tables initialized");
}

async function createExpense({ employeeName, amount, category, description }) {
  const id = crypto.randomUUID();
  const now = new Date().toISOString();
  const parsedAmount = parseFloat(amount);

  await pool.query(
    `INSERT INTO expenses (id, employee_name, amount, category, description, status, created_at, updated_at)
     VALUES ($1, $2, $3, $4, $5, 'pending', $6, $6)`,
    [id, employeeName, parsedAmount, category, description || null, now]
  );

  return {
    id,
    employeeName,
    amount: parsedAmount,
    category,
    description,
    status: "pending",
    quorumRequestId: null,
    approvals: [],
    createdAt: now,
    updatedAt: now,
  };
}

async function getExpenseById(id) {
  const { rows } = await pool.query(
    `SELECT id, employee_name, amount, category, description, status, quorum_request_id, approvals, created_at, updated_at
     FROM expenses WHERE id = $1`,
    [id]
  );
  return rows.length ? toExpenseObj(rows[0]) : null;
}

async function getExpenseByQuorumId(quorumId) {
  const { rows } = await pool.query(
    `SELECT id, employee_name, amount, category, description, status, quorum_request_id, approvals, created_at, updated_at
     FROM expenses WHERE quorum_request_id = $1`,
    [quorumId]
  );
  return rows.length ? toExpenseObj(rows[0]) : null;
}

async function listExpenses() {
  const { rows } = await pool.query(
    `SELECT id, employee_name, amount, category, description, status, quorum_request_id, approvals, created_at, updated_at
     FROM expenses ORDER BY created_at DESC`
  );
  return rows.map(toExpenseObj);
}

async function updateExpense(id, fields) {
  const sets = [];
  const values = [];
  let i = 1;

  if (fields.status !== undefined) {
    sets.push(`status = $${i++}`);
    values.push(fields.status);
  }
  if (fields.quorumRequestId !== undefined) {
    sets.push(`quorum_request_id = $${i++}`);
    values.push(fields.quorumRequestId);
  }
  if (fields.approvals !== undefined) {
    sets.push(`approvals = $${i++}`);
    values.push(JSON.stringify(fields.approvals));
  }
  sets.push(`updated_at = $${i++}`);
  values.push(new Date().toISOString());

  values.push(id);
  await pool.query(
    `UPDATE expenses SET ${sets.join(", ")} WHERE id = $${i}`,
    values
  );
}

function toExpenseObj(row) {
  return {
    id: row.id,
    employeeName: row.employee_name,
    amount: parseFloat(row.amount),
    category: row.category,
    description: row.description,
    status: row.status,
    quorumRequestId: row.quorum_request_id,
    approvals: row.approvals || [],
    createdAt: row.created_at?.toISOString?.() || row.created_at,
    updatedAt: row.updated_at?.toISOString?.() || row.updated_at,
  };
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
      "X-Tenant-ID": "expenses",
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

app.use(
  express.json({
    verify: (req, _res, buf) => {
      req.rawBody = buf;
    },
  })
);
app.use(express.urlencoded({ extended: true }));

app.set("view engine", "ejs");
app.set("views", path.join(__dirname, "views"));

// ---------------------------------------------------------------------------
// Routes
// ---------------------------------------------------------------------------

// Dashboard
app.get("/", async (_req, res) => {
  const allExpenses = await listExpenses();
  res.render("dashboard", { expenses: allExpenses });
});

// New expense form
app.get("/expense/new", (_req, res) => {
  res.render("new_expense", { error: null });
});

// Create expense via form
app.post("/expense/new", async (req, res) => {
  const { employeeName, amount, category, description } = req.body;

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

  const expense = await createExpense({ employeeName, amount, category, description });

  try {
    const result = await submitRequest(expense, employeeName.toLowerCase().replace(/\s+/g, "."));
    if (result.status === 201 && result.data) {
      await updateExpense(expense.id, { quorumRequestId: result.data.id });
      expense.quorumRequestId = result.data.id;
      console.log(`[expense] Created expense ${expense.id}, Quorum request: ${result.data.id}`);
    } else {
      console.error(`[expense] Failed to submit to Quorum: ${result.status}`, result.raw);
      await updateExpense(expense.id, { status: "error" });
    }
  } catch (err) {
    console.error("[expense] Error submitting to Quorum:", err.message);
    await updateExpense(expense.id, { status: "error" });
  }

  res.redirect(`/expense/${expense.id}`);
});

// Create expense via JSON API (used by seed script)
app.post("/api/expenses", async (req, res) => {
  const { employeeName, amount, category, description } = req.body;

  if (!employeeName || !amount || !category) {
    return res.status(400).json({ error: "employeeName, amount, and category are required" });
  }

  const expense = await createExpense({ employeeName, amount, category, description });

  try {
    const result = await submitRequest(expense, employeeName.toLowerCase().replace(/\s+/g, "."));
    if (result.status === 201 && result.data) {
      await updateExpense(expense.id, { quorumRequestId: result.data.id });
      expense.quorumRequestId = result.data.id;
      console.log(`[expense] Created expense ${expense.id}, Quorum request: ${result.data.id}`);
    } else {
      console.error(`[expense] Failed to submit to Quorum: ${result.status}`, result.raw);
      await updateExpense(expense.id, { status: "error" });
    }
  } catch (err) {
    console.error("[expense] Error submitting to Quorum:", err.message);
    await updateExpense(expense.id, { status: "error" });
  }

  res.status(201).json(expense);
});

// Expense detail
app.get("/expense/:id", async (req, res) => {
  const expense = await getExpenseById(req.params.id);
  if (!expense) {
    return res.status(404).send("Expense not found");
  }

  let quorumRequest = null;

  if (expense.quorumRequestId) {
    try {
      const result = await getRequest(expense.quorumRequestId);
      if (result.status === 200 && result.data) {
        quorumRequest = result.data;
        if (result.data.status !== expense.status) {
          await updateExpense(expense.id, { status: result.data.status });
          expense.status = result.data.status;
        }
        if (result.data.approvals) {
          await updateExpense(expense.id, { approvals: result.data.approvals });
          expense.approvals = result.data.approvals;
        }
      }
    } catch (err) {
      console.error("[expense] Error polling Quorum:", err.message);
    }
  }

  res.render("detail", { expense, quorumRequest, quorumUrl: QUORUM_PUBLIC_URL });
});

// ---------------------------------------------------------------------------
// Webhook endpoint
// ---------------------------------------------------------------------------
app.post("/webhooks/quorum", async (req, res) => {
  const signature = req.headers["x-signature-256"];

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

  let matchedExpense = await getExpenseByQuorumId(quorumReq.id);

  if (!matchedExpense) {
    const payloadExpenseId = quorumReq.payload?.expense_id;
    if (payloadExpenseId) {
      matchedExpense = await getExpenseById(payloadExpenseId);
    }
  }

  if (matchedExpense) {
    await updateExpense(matchedExpense.id, {
      status: event,
      approvals: approvals || [],
    });
    console.log(`[webhook] Updated expense ${matchedExpense.id} -> ${event}`);
  } else {
    console.warn(`[webhook] No matching expense found for Quorum request ${quorumReq.id}`);
  }

  res.status(200).json({ received: true });
});

// ---------------------------------------------------------------------------
// Startup
// ---------------------------------------------------------------------------
(async () => {
  try {
    await initDB();
  } catch (err) {
    console.error("[startup] Failed to initialize database:", err.message);
    process.exit(1);
  }

  app.listen(PORT, () => {
    console.log(`Expense Tracker running at http://localhost:${PORT}`);
    console.log(`Quorum API: ${QUORUM_API_URL}`);
    console.log(`Webhook callback: ${SELF_URL}/webhooks/quorum`);
    console.log();
  });
})();
