import { FC, useState } from "react";
import { Button, Collapse } from "react-bootstrap";
import { MdExpandLess, MdExpandMore, MdInfoOutline } from "react-icons/md";
import { OverriddenInstance, ReasonInstances } from "types/stats";
import StatsInstances from "components/StatsInstances";

interface Props {
  id: string;
  reasons: ReasonInstances[];
  overridden?: OverriddenInstance[];
}

// StatsReasons renders a "Show N reasons" toggle revealing one reason per row.
const StatsReasons: FC<Props> = ({ id, reasons, overridden }) => {
  const [show, setShow] = useState(false);

  if (reasons.length === 0) {
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
          {reasons.length} reason{reasons.length === 1 ? "" : "s"}
        </small>
      </Button>

      <Collapse in={show}>
        <div id={id} className="ms-3">
          {reasons.map((item) => (
            <div key={item.reason}>
              <small className="text-muted">{item.reason}</small>
              <StatsInstances
                id={`${id}-${item.reason}`}
                instances={item.instances}
              />
            </div>
          ))}
          {overridden && overridden.length > 0 && (
            <div>
              <small className="text-muted fst-italic">
                <MdInfoOutline size="1.15em" className="align-text-bottom" />{" "}
                Possibly unblocked by batch overrides
              </small>
              <StatsInstances id={`${id}-overridden`} instances={overridden} />
            </div>
          )}
        </div>
      </Collapse>
    </>
  );
};

export default StatsReasons;
