import { FC, useState } from "react";
import { Button, Collapse } from "react-bootstrap";
import { MdExpandLess, MdExpandMore } from "react-icons/md";
import { Link } from "react-router";
import { InstanceRef, OverriddenInstance } from "types/stats";

interface Props {
  id: string;
  instances: (InstanceRef | OverriddenInstance)[];
}

// StatsInstances renders a "Show N instances" toggle revealing one linked instance per row.
const StatsInstances: FC<Props> = ({ id, instances }) => {
  const [show, setShow] = useState(false);

  if (instances.length === 0) {
    return null;
  }

  return (
    <>
      <Button
        variant="link"
        className="p-0 ms-2 align-baseline text-decoration-none"
        onClick={() => setShow(!show)}
        aria-expanded={show}
        aria-controls={id}
      >
        <small>
          {show ? <MdExpandLess /> : <MdExpandMore />} {show ? "Hide" : "Show"}{" "}
          {instances.length} instance{instances.length === 1 ? "" : "s"}
        </small>
      </Button>

      <Collapse in={show}>
        <div id={id}>
          <div className="stats-instance-list">
            {instances.map((item) => (
              <div key={item.uuid} className="ms-3">
                <small>
                  <Link
                    to={`/ui/instances/${item.uuid}`}
                    target="_blank"
                    rel="noreferrer"
                    className="data-table-link"
                  >
                    {item.location}
                  </Link>
                  {"batch" in item && (
                    <>
                      <span className="text-muted"> by </span>
                      <Link
                        to={`/ui/batches/${item.batch}`}
                        target="_blank"
                        rel="noreferrer"
                        className="data-table-link"
                      >
                        {item.batch}
                      </Link>
                    </>
                  )}
                </small>
              </div>
            ))}
          </div>
        </div>
      </Collapse>
    </>
  );
};

export default StatsInstances;
