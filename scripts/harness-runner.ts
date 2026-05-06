#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const DEFAULT_STAGE_ORDER = ["explore", "plan", "act", "verify", "retrospect"];

function usage() {
  console.log(`Usage:
  node --experimental-strip-types scripts/harness-runner.ts --plan <plan.json> [options]

Options:
  --plan <file>             JSON plan to execute
  --run-id <id>             Run id, defaults to local timestamp
  --run-root <dir>          Artifact root, defaults to .harness/runs
  --cwd <dir>               Working directory, defaults to repo root/current cwd
  --max-corrections <n>     Correction rounds per step, defaults to 2
  --format <pretty|json|ndjson>
  --dry-run                 Return step results without executing commands
  --help                    Show this help

Plan schema:
  {
    "name": "demo",
    "objective": "make eval-search execution reproducible",
    "target": {
      "skill": "eval-search",
      "outcome": "blind search eval with scored summary"
    },
    "inputs": [
      { "id": "loader_profile", "required": true },
      { "id": "executor_profile", "required": true }
    ],
    "lifecycle": {
      "id": "eval-search",
      "stage_order": ["prepare", "understand", "plan", "act", "verify", "retrospect"]
    },
    "constraints": {
      "enforce_stage_order": true,
      "state_root": "tests/eval-search/runs",
      "role_isolation": ["loader", "executor", "judge", "optimizer"]
    },
    "artifacts": [
      { "id": "rubric", "path": "skills/eval-search/RUBRIC.md", "required": true }
    ],
    "stages": [
      {
        "id": "prepare",
        "steps": [
          {
            "id": "git_status",
            "command": ["git", "status", "--short", "--branch"],
            "expect": { "exitCode": 0 },
            "corrections": [
              { "id": "show_status", "command": ["git", "status", "--short"] }
            ]
          }
        ]
      }
    ]
  }

Every stage and step writes a structured result to the run directory. Failed
steps may run explicit correction steps, then retry themselves.`);
}

function parseArgs(argv) {
  const out: any = {
    plan: "",
    runId: "",
    runRoot: "",
    cwd: "",
    maxCorrections: 2,
    format: "pretty",
    dryRun: false,
  };
  for (let i = 0; i < argv.length; i += 1) {
    const arg = argv[i];
    const next = () => {
      if (i + 1 >= argv.length) {
        throw new Error(`missing value for ${arg}`);
      }
      i += 1;
      return argv[i];
    };
    if (arg === "--help" || arg === "-h") {
      out.help = true;
    } else if (arg === "--plan") {
      out.plan = next();
    } else if (arg === "--run-id") {
      out.runId = next();
    } else if (arg === "--run-root") {
      out.runRoot = next();
    } else if (arg === "--cwd") {
      out.cwd = next();
    } else if (arg === "--max-corrections") {
      out.maxCorrections = Number.parseInt(next(), 10);
      if (!Number.isFinite(out.maxCorrections) || out.maxCorrections < 0) {
        throw new Error("--max-corrections must be a non-negative integer");
      }
    } else if (arg === "--format") {
      out.format = next();
      if (!["pretty", "json", "ndjson"].includes(out.format)) {
        throw new Error("--format must be pretty, json, or ndjson");
      }
    } else if (arg === "--dry-run") {
      out.dryRun = true;
    } else {
      throw new Error(`unknown option ${arg}`);
    }
  }
  if (!out.help && !out.plan) {
    throw new Error("--plan is required");
  }
  return out;
}

function timestampId(date = new Date()) {
  const tzOffsetMs = date.getTimezoneOffset() * 60 * 1000;
  return new Date(date.getTime() - tzOffsetMs)
    .toISOString()
    .slice(0, 19)
    .replace(/:/g, "-");
}

function repoRoot(cwd) {
  const result = spawnSync("git", ["rev-parse", "--show-toplevel"], {
    cwd,
    encoding: "utf8",
  });
  return result.status === 0 ? result.stdout.trim() : cwd;
}

function ensureDir(dir) {
  fs.mkdirSync(dir, { recursive: true });
}

function readJson(file) {
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

function writeJson(file, value) {
  fs.writeFileSync(file, `${JSON.stringify(value, null, 2)}\n`);
}

function expandEnvVars(value, env) {
  return String(value).replace(/\$\{?([A-Z_][A-Z0-9_]*)\}?/g, (match, key) =>
    Object.prototype.hasOwnProperty.call(env, key) ? env[key] : match,
  );
}

function normalizePlan(plan) {
  if (!Array.isArray(plan.stages) || plan.stages.length === 0) {
    throw new Error("plan.stages must be a non-empty array");
  }
  return {
    name: plan.name || "harness",
    version: plan.version || 1,
    objective: plan.objective || "",
    target: normalizeObject(plan.target, "target"),
    inputs: normalizeInputs(plan.inputs || []),
    lifecycle: normalizeLifecycle(plan.lifecycle || {}, plan.objective || ""),
    constraints: normalizeConstraints(plan.constraints || {}),
    env: normalizeEnv(plan.env || {}),
    artifacts: normalizeArtifacts(plan.artifacts || []),
    stages: plan.stages.map((stage, index) => {
      if (!stage.id) {
        throw new Error(`stage at index ${index} is missing id`);
      }
      if (!Array.isArray(stage.steps) || stage.steps.length === 0) {
        throw new Error(`stage ${stage.id} must have at least one step`);
      }
      return {
        id: stage.id,
        objective: stage.objective || "",
        required: stage.required !== false,
        steps: stage.steps.map((step, stepIndex) => normalizeStep(step, stage.id, stepIndex)),
      };
    }),
  };
}

function normalizeObject(value, name) {
  if (value === undefined || value === null) {
    return {};
  }
  if (typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`plan.${name} must be an object`);
  }
  return value;
}

function normalizeLifecycle(lifecycle, objective) {
  const stageOrder = lifecycle.stage_order || lifecycle.stageOrder || DEFAULT_STAGE_ORDER;
  if (!Array.isArray(stageOrder) || stageOrder.some((stage) => typeof stage !== "string" || !stage)) {
    throw new Error("plan.lifecycle.stage_order must be a non-empty string array");
  }
  return {
    id: lifecycle.id || lifecycle.kind || "dev",
    goal: lifecycle.goal || objective || "",
    stage_order: stageOrder,
  };
}

function normalizeConstraints(constraints) {
  const out = { ...constraints };
  out.enforce_stage_order = constraints.enforce_stage_order === true || constraints.enforceStageOrder === true;
  out.state_root = constraints.state_root || constraints.stateRoot || "";
  out.role_isolation = Array.isArray(constraints.role_isolation)
    ? constraints.role_isolation
    : Array.isArray(constraints.roleIsolation)
      ? constraints.roleIsolation
      : [];
  out.allowed_write_paths = Array.isArray(constraints.allowed_write_paths)
    ? constraints.allowed_write_paths
    : Array.isArray(constraints.allowedWritePaths)
      ? constraints.allowedWritePaths
      : [];
  return out;
}

function normalizeEnv(env) {
  if (typeof env !== "object" || env === null || Array.isArray(env)) {
    throw new Error("plan.env must be an object");
  }
  return Object.fromEntries(
    Object.entries(env).map(([key, value]) => {
      if (!/^[A-Z_][A-Z0-9_]*$/.test(key)) {
        throw new Error(`plan.env key ${key} must be UPPER_SNAKE_CASE`);
      }
      return [key, String(value)];
    }),
  );
}

function normalizeArtifacts(artifacts) {
  if (!Array.isArray(artifacts)) {
    throw new Error("plan.artifacts must be an array");
  }
  return artifacts.map((artifact, index) => {
    if (!artifact.id) {
      throw new Error(`artifact at index ${index} is missing id`);
    }
    if (!artifact.path) {
      throw new Error(`artifact ${artifact.id} is missing path`);
    }
    return {
      id: artifact.id,
      path: artifact.path,
      required: artifact.required !== false,
      description: artifact.description || "",
    };
  });
}

function normalizeInputs(inputs) {
  if (!Array.isArray(inputs)) {
    throw new Error("plan.inputs must be an array");
  }
  return inputs.map((input, index) => {
    if (!input.id) {
      throw new Error(`input at index ${index} is missing id`);
    }
    return {
      id: input.id,
      required: input.required !== false,
      description: input.description || "",
      source: input.source || "",
    };
  });
}

function normalizeStep(step, stageId, index) {
  if (!step.id) {
    throw new Error(`step at ${stageId}[${index}] is missing id`);
  }
  if (!step.command) {
    throw new Error(`step ${stageId}.${step.id} is missing command`);
  }
  return {
    id: step.id,
    name: step.name || step.id,
    command: step.command,
    cwd: step.cwd || "",
    timeoutMs: step.timeout_ms || step.timeoutMs || 10 * 60 * 1000,
    required: step.required !== false,
    expect: step.expect || { exitCode: 0 },
    maxAttempts: step.max_attempts || step.maxAttempts || 1,
    corrections: Array.isArray(step.corrections)
      ? step.corrections.map((correction, correctionIndex) =>
          normalizeCorrection(correction, stageId, step.id, correctionIndex),
        )
      : [],
  };
}

function normalizeCorrection(correction, stageId, stepId, index) {
  if (!correction.id) {
    throw new Error(`correction at ${stageId}.${stepId}[${index}] is missing id`);
  }
  if (!correction.command) {
    throw new Error(`correction ${stageId}.${stepId}.${correction.id} is missing command`);
  }
  return {
    id: correction.id,
    name: correction.name || correction.id,
    command: correction.command,
    cwd: correction.cwd || "",
    timeoutMs: correction.timeout_ms || correction.timeoutMs || 10 * 60 * 1000,
    expect: correction.expect || { exitCode: 0 },
  };
}

function commandText(command) {
  return Array.isArray(command) ? command.join(" ") : command;
}

function tail(text, limit = 4000) {
  const value = String(text || "");
  return value.length <= limit ? value : value.slice(value.length - limit);
}

function runCommand(command, opts) {
  if (opts.dryRun) {
    return {
      status: 0,
      signal: null,
      stdout: "",
      stderr: "",
      error: null,
      dry_run: true,
    };
  }
  const env = { ...process.env, ...opts.env };
  if (Array.isArray(command)) {
    const [cmd, ...args] = command;
    const result = spawnSync(cmd, args, {
      cwd: opts.cwd,
      env,
      encoding: "utf8",
      timeout: opts.timeoutMs,
      maxBuffer: 64 * 1024 * 1024,
    });
    return normalizeCommandResult(result);
  }
  const result = spawnSync(command, {
    cwd: opts.cwd,
    env,
    shell: true,
    encoding: "utf8",
    timeout: opts.timeoutMs,
    maxBuffer: 64 * 1024 * 1024,
  });
  return normalizeCommandResult(result);
}

function normalizeCommandResult(result) {
  return {
    status: typeof result.status === "number" ? result.status : 1,
    signal: result.signal || null,
    stdout: result.stdout || "",
    stderr: result.stderr || "",
    error: result.error ? result.error.message : null,
    dry_run: false,
  };
}

function expectationPassed(result, expect) {
  const failures = [];
  const exitCode = expect.exitCode === undefined ? 0 : expect.exitCode;
  if (result.status !== exitCode) {
    failures.push(`exit code ${result.status}, expected ${exitCode}`);
  }
  if (expect.stdoutIncludes && !result.stdout.includes(expect.stdoutIncludes)) {
    failures.push(`stdout missing ${JSON.stringify(expect.stdoutIncludes)}`);
  }
  if (expect.stderrIncludes && !result.stderr.includes(expect.stderrIncludes)) {
    failures.push(`stderr missing ${JSON.stringify(expect.stderrIncludes)}`);
  }
  if (expect.stdoutMatches) {
    const re = new RegExp(expect.stdoutMatches);
    if (!re.test(result.stdout)) {
      failures.push(`stdout did not match /${expect.stdoutMatches}/`);
    }
  }
  return failures;
}

function classifyFailure(step, result, failures) {
  const text = `${result.stderr}\n${result.stdout}\n${result.error || ""}`;
  const actions = [];
  let category = "command_failed";
  if (result.error && /ENOENT/.test(result.error)) {
    category = "missing_command";
    actions.push(`Install or put the command on PATH: ${commandText(step.command).split(/\s+/)[0]}`);
  } else if (/command not found|not found/i.test(text)) {
    category = "missing_command";
    actions.push("Install the missing command or adjust the plan command.");
  } else if (/permission denied|not authorized|forbidden/i.test(text)) {
    category = "permission";
    actions.push("Refresh auth or request the missing permission, then retry this step.");
  } else if (/timed out|ETIMEDOUT|i\/o timeout/i.test(text)) {
    category = "timeout";
    actions.push("Retry with a larger timeout or reduce the command scope.");
  } else if (/working tree|worktree|uncommitted|dirty/i.test(`${step.id} ${text}`)) {
    category = "dirty_worktree";
    actions.push("Inspect git status and decide whether to commit, stash, or narrow the plan.");
  }
  if (step.corrections.length > 0) {
    actions.unshift("Run configured correction steps, then retry the failed step.");
  }
  if (actions.length === 0) {
    actions.push("Inspect stdout/stderr and add a targeted correction step to the plan.");
  }
  return {
    category,
    failures,
    next_actions: actions,
  };
}

function makeEmitter(format, eventsFile) {
  return function emit(event) {
    fs.appendFileSync(eventsFile, `${JSON.stringify(event)}\n`);
    if (format === "ndjson") {
      console.log(JSON.stringify(event));
    } else if (format === "pretty" && event.type === "step_result") {
      const mark = event.status === "passed" ? "PASS" : event.status === "corrected" ? "FIXED" : "FAIL";
      const retry = event.attempts > 1 ? ` attempts=${event.attempts}` : "";
      console.log(`[${mark}] ${event.stage_id}.${event.step_id}${retry} (${event.duration_ms}ms)`);
    } else if (format === "pretty" && event.type === "stage_result") {
      console.log(`[STAGE ${event.status.toUpperCase()}] ${event.stage_id}`);
    }
  };
}

function runCorrection(correction, context) {
  const cwd = path.resolve(context.cwd, correction.cwd || ".");
  const startedAt = new Date();
  const raw = runCommand(correction.command, {
    cwd,
    timeoutMs: correction.timeoutMs,
    dryRun: context.dryRun,
    env: context.env,
  });
  const endedAt = new Date();
  const failures = expectationPassed(raw, correction.expect);
  return {
    id: correction.id,
    name: correction.name,
    command: commandText(correction.command),
    cwd,
    status: failures.length === 0 ? "passed" : "failed",
    started_at: startedAt.toISOString(),
    ended_at: endedAt.toISOString(),
    duration_ms: endedAt.getTime() - startedAt.getTime(),
    exit_code: raw.status,
    signal: raw.signal,
    stdout_tail: tail(raw.stdout),
    stderr_tail: tail(raw.stderr),
    error: raw.error,
    expectation_failures: failures,
  };
}

function runStep(stage, step, context) {
  const startedAt = new Date();
  const attempts = [];
  const correctionResults = [];
  const maxAttempts = Math.max(1, step.maxAttempts + context.maxCorrections);
  let finalStatus = "failed";
  let selfCorrection = null;

  for (let attempt = 1; attempt <= maxAttempts; attempt += 1) {
    const cwd = path.resolve(context.cwd, step.cwd || ".");
    const raw = runCommand(step.command, {
      cwd,
      timeoutMs: step.timeoutMs,
      dryRun: context.dryRun,
      env: {
        ...context.env,
        HARNESS_STAGE_ID: stage.id,
        HARNESS_STEP_ID: step.id,
        HARNESS_ATTEMPT: String(attempt),
      },
    });
    const failures = expectationPassed(raw, step.expect);
    attempts.push({
      attempt,
      command: commandText(step.command),
      cwd,
      exit_code: raw.status,
      signal: raw.signal,
      stdout_tail: tail(raw.stdout),
      stderr_tail: tail(raw.stderr),
      error: raw.error,
      expectation_failures: failures,
      status: failures.length === 0 ? "passed" : "failed",
    });
    if (failures.length === 0) {
      finalStatus = attempt === 1 ? "passed" : "corrected";
      break;
    }

    selfCorrection = classifyFailure(step, raw, failures);
    if (attempt >= maxAttempts || step.corrections.length === 0) {
      break;
    }
    for (const correction of step.corrections) {
      correctionResults.push(runCorrection(correction, context));
    }
  }

  const endedAt = new Date();
  return {
    type: "step_result",
    stage_id: stage.id,
    step_id: step.id,
    name: step.name,
    required: step.required,
    status: finalStatus,
    started_at: startedAt.toISOString(),
    ended_at: endedAt.toISOString(),
    duration_ms: endedAt.getTime() - startedAt.getTime(),
    attempts: attempts.length,
    command: commandText(step.command),
    attempt_results: attempts,
    corrections: correctionResults,
    self_correction: finalStatus === "passed" ? null : selfCorrection,
  };
}

function runStage(stage, context) {
  const startedAt = new Date();
  const stepResults = [];
  let status = "passed";
  for (const step of stage.steps) {
    const result = runStep(stage, step, context);
    stepResults.push(result);
    context.emit(result);
    if (result.status === "failed" && step.required) {
      status = "failed";
      break;
    }
    if (result.status === "corrected" && status !== "failed") {
      status = "corrected";
    }
  }
  const endedAt = new Date();
  const stageResult = {
    type: "stage_result",
    stage_id: stage.id,
    objective: stage.objective,
    required: stage.required,
    status,
    started_at: startedAt.toISOString(),
    ended_at: endedAt.toISOString(),
    duration_ms: endedAt.getTime() - startedAt.getTime(),
    steps: stepResults,
  };
  context.emit({
    type: "stage_result",
    stage_id: stage.id,
    status,
    duration_ms: stageResult.duration_ms,
    failed_steps: stepResults.filter((step) => step.status === "failed").map((step) => step.step_id),
  });
  return stageResult;
}

function summarize(plan, stageResults, context, startedAt) {
  const endedAt = new Date();
  const failedStages = stageResults.filter((stage) => stage.status === "failed");
  const stageShape = validateStageShape(plan);
  const artifactResults = validateArtifacts(plan, context);
  const missingRequiredArtifacts = artifactResults.filter((artifact) => artifact.required && !artifact.exists);
  const stageOrderFailed =
    plan.constraints.enforce_stage_order &&
    (stageShape.missing.length > 0 || stageShape.unexpected.length > 0 || stageShape.out_of_order.length > 0);
  const failedSteps = stageResults.flatMap((stage) =>
    stage.steps
      .filter((step) => step.status === "failed")
      .map((step) => ({
        stage_id: stage.stage_id,
        step_id: step.step_id,
        category: step.self_correction?.category || "unknown",
        next_actions: step.self_correction?.next_actions || [],
      })),
  );
  const correctedSteps = stageResults.flatMap((stage) =>
    stage.steps
      .filter((step) => step.status === "corrected")
      .map((step) => ({ stage_id: stage.stage_id, step_id: step.step_id })),
  );
  const status =
    failedStages.length === 0 && missingRequiredArtifacts.length === 0 && !stageOrderFailed ? "passed" : "failed";
  return {
    run_id: context.runId,
    plan_name: plan.name,
    objective: plan.objective,
    target: plan.target,
    inputs: plan.inputs,
    lifecycle: plan.lifecycle,
    status,
    started_at: startedAt.toISOString(),
    ended_at: endedAt.toISOString(),
    duration_ms: endedAt.getTime() - startedAt.getTime(),
    run_dir: context.runDir,
    stage_shape: stageShape,
    artifacts: artifactResults,
    contract_failures: [
      ...(stageOrderFailed
        ? [
            {
              category: "stage_order",
              missing: stageShape.missing,
              unexpected: stageShape.unexpected,
              out_of_order: stageShape.out_of_order,
            },
          ]
        : []),
      ...missingRequiredArtifacts.map((artifact) => ({
        category: "missing_artifact",
        artifact_id: artifact.id,
        path: artifact.path,
      })),
    ],
    stages: stageResults.map((stage) => ({
      stage_id: stage.stage_id,
      status: stage.status,
      steps: stage.steps.length,
      failed_steps: stage.steps.filter((step) => step.status === "failed").length,
      corrected_steps: stage.steps.filter((step) => step.status === "corrected").length,
    })),
    failed_steps: failedSteps,
    corrected_steps: correctedSteps,
  };
}

function validateStageShape(plan) {
  const ids = plan.stages.map((stage) => stage.id);
  const expected = plan.lifecycle.stage_order;
  const missing = expected.filter((stage) => !ids.includes(stage));
  const unexpected = ids.filter((stage) => !expected.includes(stage));
  const outOfOrder = [];
  let lastIndex = -1;
  for (const id of ids) {
    const index = expected.indexOf(id);
    if (index < 0) {
      continue;
    }
    if (index < lastIndex) {
      outOfOrder.push(id);
    }
    lastIndex = Math.max(lastIndex, index);
  }
  return {
    lifecycle_id: plan.lifecycle.id,
    expected_order: expected,
    present: ids,
    missing,
    unexpected,
    out_of_order: outOfOrder,
    order_matches: missing.length === 0 && unexpected.length === 0 && outOfOrder.length === 0,
  };
}

function validateArtifacts(plan, context) {
  return plan.artifacts.map((artifact) => {
    const artifactPath = expandEnvVars(artifact.path, { ...process.env, ...context.env });
    const resolvedPath = path.isAbsolute(artifactPath) ? artifactPath : path.resolve(context.cwd, artifactPath);
    const exists = fs.existsSync(resolvedPath);
    return {
      id: artifact.id,
      path: artifact.path,
      resolved_path: resolvedPath,
      required: artifact.required,
      exists,
      status: exists ? "present" : artifact.required ? "missing" : "optional_missing",
    };
  });
}

function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    usage();
    return;
  }
  const baseCwd = args.cwd ? path.resolve(args.cwd) : repoRoot(process.cwd());
  const runId = args.runId || timestampId();
  const runRoot = path.resolve(baseCwd, args.runRoot || ".harness/runs");
  const runDir = path.join(runRoot, runId);
  const stagesDir = path.join(runDir, "stages");
  ensureDir(stagesDir);

  const planPath = path.resolve(baseCwd, args.plan);
  const plan = normalizePlan(readJson(planPath));
  const eventsFile = path.join(runDir, "events.ndjson");
  fs.writeFileSync(eventsFile, "");
  const emit = makeEmitter(args.format, eventsFile);
  const startedAt = new Date();
  const context = {
    cwd: baseCwd,
    runId,
    runDir,
    dryRun: args.dryRun,
    maxCorrections: args.maxCorrections,
    emit,
    env: {
      ...plan.env,
      HARNESS_RUN_ID: runId,
      HARNESS_RUN_DIR: runDir,
      HARNESS_PLAN: plan.name,
    },
  };

  writeJson(path.join(runDir, "plan.json"), plan);
  writeJson(path.join(runDir, "stage_shape.json"), validateStageShape(plan));
  writeJson(path.join(runDir, "contract.json"), {
    objective: plan.objective,
    target: plan.target,
    inputs: plan.inputs,
    lifecycle: plan.lifecycle,
    constraints: plan.constraints,
    artifacts: plan.artifacts,
    env: plan.env,
  });
  emit({
    type: "run_started",
    run_id: runId,
    plan_name: plan.name,
    objective: plan.objective,
    target: plan.target,
    inputs: plan.inputs,
    lifecycle: plan.lifecycle,
    cwd: baseCwd,
    run_dir: runDir,
    dry_run: args.dryRun,
  });

  const stageResults = [];
  for (const stage of plan.stages) {
    const result = runStage(stage, context);
    stageResults.push(result);
    writeJson(path.join(stagesDir, `${stage.id}.json`), result);
    if (result.status === "failed" && stage.required) {
      break;
    }
  }

  const summary = summarize(plan, stageResults, context, startedAt);
  writeJson(path.join(runDir, "summary.json"), summary);
  emit({ type: "run_finished", ...summary });
  if (args.format === "json") {
    console.log(JSON.stringify(summary, null, 2));
  } else if (args.format === "pretty") {
    console.log(JSON.stringify(summary, null, 2));
  }
  if (summary.status !== "passed") {
    process.exitCode = 1;
  }
}

try {
  main();
} catch (err) {
  console.error(JSON.stringify({ ok: false, error: err.message }, null, 2));
  process.exitCode = 1;
}
