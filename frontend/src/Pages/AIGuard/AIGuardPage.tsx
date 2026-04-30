import { useMemo, useState } from "react";
import { observer } from "mobx-react-lite";
import LeftPanel from "../LeftPanel/LeftPanel";
import { serverMetricsStore } from "../../Store/ServerMetricsStore";
import {
  analyzeCommandRisk,
  guardedExecute,
  loadCommandAudit,
  type RiskAnalysisResponse,
} from "../../Api/features/aiGuard";
import styles from "./AIGuardPage.module.scss";

const AIGuardPage = observer(() => {
  const servers = serverMetricsStore.getNowServers();
  const [serverId, setServerId] = useState<number>(servers[0]?.id ?? 0);
  const [command, setCommand] = useState("");
  const [analysis, setAnalysis] = useState<RiskAnalysisResponse | null>(null);
  const [trace, setTrace] = useState<string[]>([]);
  const [audit, setAudit] = useState<
    Array<{
      id: number;
      command: string;
      risk_level: string;
      decision: string;
      created_at: string;
    }>
  >([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditFilter, setAuditFilter] = useState<
    "all" | "low" | "medium" | "high" | "critical"
  >("all");

  const onAnalyze = async () => {
    setError("");
    setLoading(true);
    try {
      setAnalysis(await analyzeCommandRisk({ command, server_id: serverId }));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const onExecute = async () => {
    if (!analysis?.requires_confirmation || analysis.blocked) return;
    if (
      !window.confirm("Explicit confirmation required. Execute this command?")
    )
      return;
    setLoading(true);
    try {
      const res = await guardedExecute({
        command,
        server_id: serverId,
        risk_ack: true,
        reason: "manual confirmed",
      });
      setTrace(res.trace || []);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const onLoadAudit = async () => {
    setAuditLoading(true);
    try {
      setAudit(await loadCommandAudit(serverId));
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setAuditLoading(false);
    }
  };
  const quickCommands = [
    "systemctl restart nginx",
    "docker ps -a",
    "rm -rf /tmp/cache/*",
  ];
  const filteredAudit = useMemo(
    () =>
      audit.filter((item) =>
        auditFilter === "all" ? true : item.risk_level === auditFilter,
      ),
    [audit, auditFilter],
  );

  return (
    <div className={styles.page}>
      <LeftPanel />
      <main className={styles.main}>
        <header className={styles.header}>
          <h1>AI Command Guard</h1>
        </header>
        <section className={styles.card}>
          <h2>Command Preview</h2>
          <div className={styles.row}>
            <select
              value={serverId}
              onChange={(e) => setServerId(Number(e.target.value))}
            >
              {servers.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
            <input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder="Command to analyze before execution"
            />
            <button
              onClick={() => void onAnalyze()}
              disabled={!command || loading}
            >
              Analyze Risk
            </button>
          </div>
          <div className={styles.row}>
            {quickCommands.map((item) => (
              <button key={item} type="button" onClick={() => setCommand(item)}>
                {item}
              </button>
            ))}
          </div>
          {loading ? <p>Loading…</p> : null}
          {error ? <p className={styles.error}>{error}</p> : null}
        </section>
        <section className={styles.grid}>
          <article className={styles.card}>
            <h2>Risk Panel</h2>
            {analysis ? (
              <>
                <p>
                  Risk level: <strong>{analysis.risk_level}</strong>
                </p>
                <p>{analysis.summary}</p>
                <p>Blocked: {analysis.blocked ? "yes" : "no"}</p>
                <p>
                  Needs confirmation:{" "}
                  {analysis.requires_confirmation ? "yes" : "no"}
                </p>
                <p>
                  Policy matches:{" "}
                  {analysis.policy_matches.length
                    ? analysis.policy_matches.join(", ")
                    : "none"}
                </p>
                <ul>
                  {analysis.findings.length === 0 ? (
                    <li>No findings.</li>
                  ) : (
                    analysis.findings.map((f) => (
                      <li key={f.code}>
                        {f.title} — {f.details}
                      </li>
                    ))
                  )}
                </ul>
              </>
            ) : (
              <p>Run analysis to see risk details.</p>
            )}
          </article>
          <article className={styles.card}>
            <h2>Sandbox Preview</h2>
            {analysis ? (
              <ul>
                {analysis.sandbox_preview.map((p) => (
                  <li key={p}>{p}</li>
                ))}
              </ul>
            ) : (
              <p>Empty.</p>
            )}
            <button
              onClick={() => void onExecute()}
              disabled={!analysis || analysis.blocked || loading}
            >
              Confirm & Execute
            </button>
          </article>
          <article className={styles.card}>
            <h2>Execution Trace</h2>
            {trace.length ? <pre>{trace.join("\n")}</pre> : <p>Empty.</p>}
          </article>
          <article className={styles.card}>
            <h2>Audit</h2>
            <div className={styles.row}>
              <button
                onClick={() => void onLoadAudit()}
                disabled={auditLoading}
              >
                {auditLoading ? "Loading…" : "Load audit logs"}
              </button>
              <select
                value={auditFilter}
                onChange={(e) =>
                  setAuditFilter(
                    e.target.value as
                      | "all"
                      | "low"
                      | "medium"
                      | "high"
                      | "critical",
                  )
                }
              >
                <option value="all">All risk levels</option>
                <option value="low">low</option>
                <option value="medium">medium</option>
                <option value="high">high</option>
                <option value="critical">critical</option>
              </select>
            </div>
            {filteredAudit.length === 0 ? (
              <p>No audit entries for this filter.</p>
            ) : (
              filteredAudit.map((a) => (
                <div key={a.id}>
                  {new Date(a.created_at).toLocaleString()} • {a.risk_level} •{" "}
                  {a.decision} • {a.command}
                </div>
              ))
            )}
          </article>
        </section>
      </main>
    </div>
  );
});

export default AIGuardPage;
