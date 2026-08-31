import { FC, ReactNode, useState } from "react";
import { Button, Collapse } from "react-bootstrap";
import { MdExpandLess, MdExpandMore } from "react-icons/md";
import { maxWidth } from "util/stats";

interface Props {
  count: number;
  singular: string;
  plural: string;
  children: ReactNode;
}

// StatsCollapsibleList renders a "Show N items" toggle above a collapsible list.
const StatsCollapsibleList: FC<Props> = ({
  count,
  singular,
  plural,
  children,
}) => {
  const [show, setShow] = useState(false);

  if (count === 0) {
    return null;
  }

  return (
    <>
      <div className="container" style={{ maxWidth }}>
        <Button
          variant="link"
          className="ps-0 mt-2 text-decoration-none"
          onClick={() => setShow(!show)}
          aria-expanded={show}
        >
          <small>
            {show ? <MdExpandLess /> : <MdExpandMore />}{" "}
            {show ? "Hide" : "Show"} {count} {count === 1 ? singular : plural}
          </small>
        </Button>
      </div>

      <Collapse in={show}>
        <div>{children}</div>
      </Collapse>
    </>
  );
};

export default StatsCollapsibleList;
