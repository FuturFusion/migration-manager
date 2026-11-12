import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import {
  QueryClient,
  QueryClientProvider,
  useQuery,
} from "@tanstack/react-query";
import { afterEach, expect, test, vi } from "vitest";
import DataTable from "components/DataTable";
import FileUploader from "components/FileUploader";
import InstanceStateModal from "components/InstanceStateModal";
import { changePowerState } from "api/instances";
import BatchActions from "components/BatchActions";
import BatchResetBtn from "components/BatchResetBtn";
import QueueActions from "components/QueueActions";
import QueueCancelBtn from "components/QueueCancelBtn";
import { Batch } from "types/batch";
import { QueueEntry } from "types/queue";
import { cancelQueue, deleteQueue } from "api/queue";
import { startBatch } from "api/batches";

vi.mock("api/instances", () => ({ changePowerState: vi.fn() }));
vi.mock("api/queue", () => ({ cancelQueue: vi.fn(), deleteQueue: vi.fn() }));
vi.mock("api/batches", () => ({ startBatch: vi.fn() }));

const batch = { name: "batch-1", status: "Running" } as Batch;
const queueEntry = {
  instance_uuid: "vm-1",
  migration_status: "Waiting",
} as QueueEntry;

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>
    {children}
  </QueryClientProvider>
);

afterEach(() => {
  vi.resetAllMocks();
  document.documentElement.removeAttribute("data-bs-theme");
});

test.each(["light", "dark"])(
  "batch action availability and keyboard explanations in %s mode",
  async (theme) => {
    document.documentElement.setAttribute("data-bs-theme", theme);
    render(<BatchActions batch={batch} />, { wrapper });
    const start = screen.getByRole("button", { name: "Start batch" });
    expect(start).toBeDisabled();
    expect(screen.getByRole("button", { name: "Stop batch" })).toBeEnabled();
    fireEvent.focus(start.parentElement!);
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Cannot start a batch in state Running.",
    );
    fireEvent.click(start);
    expect(startBatch).not.toHaveBeenCalled();
  },
);

test("a pending batch request disables sibling actions and allows retry after rejection", async () => {
  let complete!: (response: Awaited<ReturnType<typeof startBatch>>) => void;
  vi.mocked(startBatch).mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        complete = resolve;
      }),
  );
  render(<BatchActions batch={{ ...batch, status: "Defined" }} />, { wrapper });
  const start = screen.getByRole("button", { name: "Start batch" });
  fireEvent.click(start);
  expect(start).toBeDisabled();
  expect(start).toHaveAttribute("aria-busy", "true");
  expect(screen.getByText("Starting…")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Delete batch" })).toBeDisabled();
  fireEvent.click(start);
  expect(startBatch).toHaveBeenCalledTimes(1);
  await act(async () =>
    complete({ error_code: 409, error: "Batch changed" } as Awaited<
      ReturnType<typeof startBatch>
    >),
  );
  expect(start).toBeEnabled();
  expect(screen.getByRole("button", { name: "Delete batch" })).toBeEnabled();
});

test("a reset dialog explains a state change and disables submission", () => {
  const { rerender } = render(<BatchResetBtn batch={batch} />, { wrapper });
  fireEvent.click(screen.getByRole("button", { name: "Reset batch" }));
  rerender(<BatchResetBtn batch={{ ...batch, status: "Defined" }} />);
  const dialog = screen.getByRole("dialog");
  expect(within(dialog).getByRole("alert")).toHaveTextContent(
    "there is nothing to reset",
  );
  expect(
    within(dialog).getByRole("button", { name: "Reset batch" }),
  ).toBeDisabled();
});

test("cancel shows progress, prevents duplicates, preserves options on failure, and closes after retry", async () => {
  let rejectRequest!: (error: Error) => void;
  vi.mocked(cancelQueue).mockImplementationOnce(
    () =>
      new Promise((_, reject) => {
        rejectRequest = reject;
      }),
  );
  render(<QueueCancelBtn queueEntry={queueEntry} />, { wrapper });
  fireEvent.click(screen.getByRole("button", { name: "Cancel migration" }));
  const dialog = screen.getByRole("dialog");
  const cleanup = within(dialog).getByRole("checkbox");
  fireEvent.click(cleanup);
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Cancel migration" }),
  );
  expect(
    within(dialog).getByRole("button", { name: "Cancelling…" }),
  ).toBeDisabled();
  expect(cleanup).toBeDisabled();
  expect(
    within(dialog).queryByRole("button", { name: "Close" }),
  ).not.toBeInTheDocument();
  fireEvent.keyDown(dialog, { key: "Escape", code: "Escape", keyCode: 27 });
  fireEvent.click(within(dialog).getByRole("button", { name: "Cancelling…" }));
  expect(cancelQueue).toHaveBeenCalledTimes(1);
  expect(cancelQueue).toHaveBeenCalledWith("vm-1", true);
  await act(async () => rejectRequest(new Error("Connection lost")));
  expect(within(dialog).getByRole("alert")).toHaveTextContent(
    "Connection lost",
  );
  expect(cleanup).toBeChecked();
  expect(cleanup).toBeEnabled();
  vi.mocked(cancelQueue).mockResolvedValueOnce({ error_code: 0 } as Awaited<
    ReturnType<typeof cancelQueue>
  >);
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Cancel migration" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
  expect(cancelQueue).toHaveBeenCalledTimes(2);
});

test("cancel submission becomes unavailable when a migration is canceled elsewhere", () => {
  const { rerender } = render(<QueueCancelBtn queueEntry={queueEntry} />, {
    wrapper,
  });
  fireEvent.click(screen.getByRole("button", { name: "Cancel migration" }));
  rerender(
    <QueueCancelBtn
      queueEntry={{ ...queueEntry, migration_status: "Canceled" }}
    />,
  );
  const dialog = screen.getByRole("dialog");
  expect(within(dialog).getByRole("alert")).toHaveTextContent(
    "already canceled",
  );
  expect(
    within(dialog).getByRole("button", { name: "Cancel migration" }),
  ).toBeDisabled();
  expect(cancelQueue).not.toHaveBeenCalled();
});

test("queue deletion retains API errors and closes after a successful retry", async () => {
  vi.mocked(deleteQueue).mockResolvedValueOnce({
    error_code: 409,
    error: "Entry is in use",
  } as Awaited<ReturnType<typeof deleteQueue>>);
  render(
    <QueueActions
      queueEntry={{ ...queueEntry, migration_status: "Finished" }}
    />,
    { wrapper },
  );
  fireEvent.click(screen.getByRole("button", { name: "Delete queue entry" }));
  fireEvent.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }),
  );
  expect(await screen.findByRole("alert")).toHaveTextContent("Entry is in use");
  vi.mocked(deleteQueue).mockResolvedValueOnce({ error_code: 0 } as Awaited<
    ReturnType<typeof deleteQueue>
  >);
  fireEvent.click(
    within(screen.getByRole("dialog")).getByRole("button", { name: "Delete" }),
  );
  await waitFor(() =>
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
  );
});

test("a refreshed table keeps an open dialog attached to its original batch", () => {
  const first = { ...batch, name: "first" };
  const second = { ...batch, name: "second" };
  const rows = (items: Batch[]) =>
    items.map((item) => ({
      key: item.name,
      cols: [
        { content: item.name },
        { content: <BatchActions batch={item} /> },
      ],
    }));
  const { rerender } = render(
    <DataTable headers={["Name", "Actions"]} rows={rows([first, second])} />,
    { wrapper },
  );
  fireEvent.click(screen.getAllByRole("button", { name: "Reset batch" })[0]);
  rerender(
    <DataTable headers={["Name", "Actions"]} rows={rows([second, first])} />,
  );
  expect(screen.getByRole("dialog")).toHaveTextContent(
    'reset the batch "first"',
  );
});

test("uploads recover from rejection and cannot reuse a cleared selection", async () => {
  const onUpload = vi.fn().mockRejectedValueOnce(new Error("Connection lost"));
  render(<FileUploader onUpload={onUpload} />);
  const input = screen.getByLabelText("File to upload");
  const file = new File(["data"], "backup.tar");
  fireEvent.change(input, { target: { files: [file] } });
  fireEvent.click(screen.getByRole("button", { name: "Upload" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Connection lost");
  expect(screen.getByRole("button", { name: "Upload" })).toBeEnabled();
  fireEvent.change(input, { target: { files: [] } });
  expect(screen.getByRole("button", { name: "Upload" })).toBeDisabled();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  expect(onUpload).toHaveBeenCalledTimes(1);
});

test("power confirmation cannot reverse its action when the instance state changes", () => {
  const props = {
    uuid: "vm-1",
    running: false,
    show: true,
    handleClose: vi.fn(),
  };
  const { rerender } = render(<InstanceStateModal {...props} />, { wrapper });
  expect(screen.getByRole("button", { name: "Power on" })).toBeEnabled();
  rerender(<InstanceStateModal {...props} running />);
  expect(screen.getByRole("button", { name: "Power on" })).toBeDisabled();
  expect(screen.getByRole("alert")).toHaveTextContent("power state changed");
  expect(changePowerState).not.toHaveBeenCalled();
});

test("power changes invalidate instance lists, details, and batch instance views", async () => {
  const client = new QueryClient();
  const keys = [
    ["instances"],
    ["instances", "vm-1"],
    ["batches", "batch-1", "instances"],
  ];
  keys.forEach((key) => client.setQueryData<unknown[]>(key, []));
  vi.mocked(changePowerState).mockResolvedValueOnce({
    error_code: 0,
  } as Awaited<ReturnType<typeof changePowerState>>);
  render(
    <QueryClientProvider client={client}>
      <InstanceStateModal
        uuid="vm-1"
        running={false}
        show
        handleClose={vi.fn()}
      />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Power on" }));
  await waitFor(() =>
    keys.forEach((key) =>
      expect(client.getQueryState(key)?.isInvalidated).toBe(true),
    ),
  );
});

test("batch controls stay busy until the accepted action's state has refreshed", async () => {
  let finishRefresh!: (value: Batch) => void;
  const refresh = vi.fn(
    () =>
      new Promise<Batch>((resolve) => {
        finishRefresh = resolve;
      }),
  );
  const client = new QueryClient();
  const BatchRow = () => {
    const { data } = useQuery({
      queryKey: ["batches", batch.name],
      queryFn: refresh,
      initialData: { ...batch, status: "Defined" },
      staleTime: Infinity,
    });
    return <BatchActions batch={data} />;
  };
  vi.mocked(startBatch).mockResolvedValueOnce({ error_code: 0 } as Awaited<
    ReturnType<typeof startBatch>
  >);
  render(
    <QueryClientProvider client={client}>
      <BatchRow />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Start batch" }));
  await waitFor(() => expect(refresh).toHaveBeenCalledTimes(1));
  const start = screen.getByRole("button", { name: "Start batch" });
  expect(start).toBeDisabled();
  expect(start).toHaveAttribute("aria-busy", "true");
  fireEvent.click(start);
  expect(startBatch).toHaveBeenCalledTimes(1);
  await act(async () => finishRefresh(batch));
  await waitFor(() =>
    expect(screen.getByRole("button", { name: "Stop batch" })).toBeEnabled(),
  );
  expect(screen.getByRole("button", { name: "Start batch" })).toBeDisabled();
});

test.each(["reset", "cancel"])(
  "reopening the %s dialog clears destructive options",
  async (action) => {
    render(
      action === "reset" ? (
        <BatchResetBtn batch={batch} />
      ) : (
        <QueueCancelBtn queueEntry={queueEntry} />
      ),
      { wrapper },
    );
    const label = action === "reset" ? "Reset batch" : "Cancel migration";
    fireEvent.click(screen.getByRole("button", { name: label }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("checkbox"));
    expect(
      within(screen.getByRole("dialog")).getByRole("checkbox"),
    ).toBeChecked();
    fireEvent.click(
      within(screen.getByRole("dialog")).getByRole("button", { name: "Close" }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: label }));
    expect(
      within(screen.getByRole("dialog")).getByRole("checkbox"),
    ).not.toBeChecked();
  },
);

test("a finished migration offers deletion but neither cancellation nor retry", async () => {
  render(
    <QueueActions
      queueEntry={{ ...queueEntry, migration_status: "Finished" }}
    />,
    { wrapper },
  );
  const cancel = screen.getByRole("button", { name: "Cancel migration" });
  expect(cancel).toBeDisabled();
  expect(
    screen.getByRole("button", { name: "Retry migration" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("button", { name: "Delete queue entry" }),
  ).toBeEnabled();
  fireEvent.focus(cancel.parentElement!);
  expect(await screen.findByRole("tooltip")).toHaveTextContent(
    "Migration is already finished.",
  );
  fireEvent.click(cancel);
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
});

test("only a running batch can be stopped", () => {
  const { rerender } = render(
    <BatchActions batch={{ ...batch, status: "Queued" }} />,
    { wrapper },
  );
  expect(screen.getByRole("button", { name: "Stop batch" })).toBeDisabled();
  rerender(<BatchActions batch={batch} />);
  expect(screen.getByRole("button", { name: "Stop batch" })).toBeEnabled();
});
