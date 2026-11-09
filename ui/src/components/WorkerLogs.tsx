import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Alert, Button, Form } from "react-bootstrap";
import { BsDownload } from "react-icons/bs";
import { useParams } from "react-router";
import { fetchInstance } from "api/instances";
import {
  fetchWorkerLog,
  fetchWorkerLogSessions,
  workerLogDownloadURL,
} from "api/workerLogs";

const WorkerLogs = () => {
  const { uuid = "" } = useParams<{ uuid: string }>();
  const [selectedSession, setSelectedSession] = useState("");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [autoScroll, setAutoScroll] = useState(true);
  const [oldestFirst, setOldestFirst] = useState(false);
  const [lineInput, setLineInput] = useState("200");
  const [lines, setLines] = useState(200);
  const output = useRef<HTMLPreElement>(null);
  const interval = autoRefresh ? 5000 : false;

  const instance = useQuery({
    queryKey: ["instances", uuid],
    queryFn: () => fetchInstance(uuid),
  });
  const sessions = useQuery({
    queryKey: ["worker-log-sessions", uuid],
    queryFn: ({ signal }) => fetchWorkerLogSessions(uuid, signal),
    refetchInterval: interval,
    refetchOnWindowFocus: autoRefresh,
    refetchOnReconnect: autoRefresh,
  });
  const workerSessions = [...(sessions.data ?? [])].sort(
    (a, b) => Date.parse(b.modified) - Date.parse(a.modified),
  );
  const session =
    workerSessions.find((item) => item.session === selectedSession)?.session ||
    workerSessions[0]?.session ||
    "";
  const log = useQuery({
    queryKey: ["worker-logs", uuid, session, lines, oldestFirst],
    queryFn: ({ signal }) =>
      fetchWorkerLog(uuid, session, lines, oldestFirst, signal),
    enabled: session !== "",
    refetchInterval: interval,
    refetchOnWindowFocus: autoRefresh,
    refetchOnReconnect: autoRefresh,
  });

  useEffect(() => {
    if (autoScroll && output.current) {
      output.current.scrollTop = oldestFirst ? 0 : output.current.scrollHeight;
    }
  }, [autoScroll, log.data, oldestFirst]);

  return (
    <div>
      <h6 className="mb-3">Worker session for {instance.data?.name || uuid}</h6>
      {sessions.error && (
        <Alert variant="danger">Failed to load worker log sessions.</Alert>
      )}
      {sessions.isPending && <div>Loading...</div>}
      {sessions.isSuccess && !workerSessions.length && (
        <Alert variant="info">No worker logs have been received yet.</Alert>
      )}
      {session && (
        <div className="d-flex flex-wrap align-items-center gap-3 mb-3">
          <Form.Group className="mw-100" controlId="worker-log-session">
            <Form.Select
              className="w-auto mw-100"
              aria-label="Worker session"
              value={session}
              onChange={(event) => setSelectedSession(event.target.value)}
            >
              {workerSessions.map((item) => (
                <option key={item.session} value={item.session}>
                  {new Date(item.modified).toLocaleString()} — {item.session}
                </option>
              ))}
            </Form.Select>
          </Form.Group>
          <a
            className="d-inline-flex align-items-center gap-2"
            href={workerLogDownloadURL(uuid, session)}
            download
          >
            <BsDownload aria-hidden="true" />
            Download logs
          </a>
        </div>
      )}
      <Form
        className="d-flex flex-wrap align-items-end gap-3 mb-3"
        onSubmit={(event) => {
          event.preventDefault();
          const nextLines = Number(lineInput);
          void sessions.refetch();
          if (nextLines !== lines) {
            setLines(nextLines);
          } else if (session) {
            void log.refetch();
          }
        }}
      >
        <Form.Group controlId="worker-log-lines">
          <Form.Label>Lines</Form.Label>
          <Form.Control
            type="number"
            min={1}
            max={10000}
            step={1}
            required
            value={lineInput}
            onChange={(event) => setLineInput(event.target.value)}
          />
        </Form.Group>
        <div className="d-flex flex-wrap align-items-center gap-3">
          <Button type="submit" variant="outline-primary">
            Refresh
          </Button>
          <Form.Check
            id="worker-log-auto-refresh"
            label="Auto-refresh"
            checked={autoRefresh}
            onChange={(event) => setAutoRefresh(event.target.checked)}
          />
          <Form.Check
            id="worker-log-auto-scroll"
            label="Auto-scroll"
            checked={autoScroll}
            onChange={(event) => setAutoScroll(event.target.checked)}
          />
          <Form.Check
            id="worker-log-oldest-first"
            label="Oldest first"
            checked={oldestFirst}
            onChange={(event) => setOldestFirst(event.target.checked)}
          />
        </div>
      </Form>
      {log.error && <Alert variant="danger">Failed to load worker logs.</Alert>}
      {session && log.isPending && <div>Loading...</div>}
      {log.data && (
        <pre
          ref={output}
          className="border rounded p-3 overflow-auto"
          style={{ maxHeight: "60vh", whiteSpace: "pre-wrap" }}
          aria-label="Worker log output"
        >
          {log.data.content || "No log output."}
        </pre>
      )}
    </div>
  );
};

export default WorkerLogs;
