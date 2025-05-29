import { FC, useEffect, useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import { BsPlus, BsTrash } from "react-icons/bs";
import { BatchConstraint } from "types/batch";

interface Props {
  value: BatchConstraint[];
  onChange: (value: BatchConstraint[]) => void;
}

const INITIAL_CONSTRAINT = {
  name: "",
  description: "",
  include_expression: "",
  max_concurrent_instances: 0,
  min_instance_boot_time: "",
};

const BatchConstraintsWidget: FC<Props> = ({ value, onChange }) => {
  const [entries, setEntries] = useState<BatchConstraint[]>(value || []);
  const [constraint, setConstraint] = useState<BatchConstraint>({
    ...INITIAL_CONSTRAINT,
  });

  const handleAdd = () => {
    const newValues = [...entries, constraint];
    setEntries(newValues);
    onChange(newValues);
    setConstraint({ ...INITIAL_CONSTRAINT });
  };

  useEffect(() => {
    setEntries(value || []);
  }, [value]);

  const handleDelete = (index: number) => {
    const updated = entries.filter((_, idx) => idx != index);
    setEntries(updated);
    onChange(updated);
  };

  function updateConstraintField<T, K extends keyof T>(
    obj: T,
    key: K,
    value: T[K],
  ) {
    obj[key] = value;
  }

  const handleEdit = (index: number, field: string, value: string | number) => {
    const newValue = entries[index];
    updateConstraintField(newValue, field as keyof BatchConstraint, value);

    const newValues = entries.map((item, idx) =>
      idx === index ? newValue : item,
    );
    setEntries(newValues);
    onChange(newValues);
  };

  return (
    <div>
      <Table borderless>
        <tbody>
          {entries.map((item, index) => (
            <>
              <tr key={index}>
                <td style={{ display: "flex", gap: "8px" }}>
                  <Form.Control
                    type="text"
                    size="sm"
                    value={item.name}
                    onChange={(e) => handleEdit(index, "name", e.target.value)}
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    value={item.description}
                    onChange={(e) =>
                      handleEdit(index, "description", e.target.value)
                    }
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    value={item.include_expression}
                    onChange={(e) =>
                      handleEdit(index, "include_expression", e.target.value)
                    }
                  />
                  <Form.Control
                    type="number"
                    size="sm"
                    value={item.max_concurrent_instances}
                    onChange={(e) =>
                      handleEdit(
                        index,
                        "max_concurrent_instances",
                        e.target.value,
                      )
                    }
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    value={item.min_instance_boot_time}
                    onChange={(e) =>
                      handleEdit(
                        index,
                        "min_instance_boot_time",
                        e.target.value,
                      )
                    }
                  />
                </td>
                <td>
                  <Button
                    title="Delete"
                    size="sm"
                    variant="outline-secondary"
                    className="bg-white border no-hover"
                    onClick={() => handleDelete(index)}
                  >
                    <BsTrash />
                  </Button>
                </td>
              </tr>
            </>
          ))}
          <tr>
            <td style={{ display: "flex", gap: "8px" }}>
              <Form.Control
                type="text"
                size="sm"
                placeholder="Name"
                value={constraint.name}
                onChange={(e) =>
                  setConstraint({ ...constraint, name: e.target.value })
                }
              />
              <Form.Control
                type="text"
                size="sm"
                placeholder="Description"
                value={constraint.description}
                onChange={(e) =>
                  setConstraint({ ...constraint, description: e.target.value })
                }
              />
              <Form.Control
                type="text"
                size="sm"
                placeholder="Include expression"
                value={constraint.include_expression}
                onChange={(e) =>
                  setConstraint({
                    ...constraint,
                    include_expression: e.target.value,
                  })
                }
              />
              <Form.Control
                type="number"
                size="sm"
                placeholder="Max concurrent instances"
                value={constraint.max_concurrent_instances}
                onChange={(e) =>
                  setConstraint({
                    ...constraint,
                    max_concurrent_instances: Number(e.target.value),
                  })
                }
              />
              <Form.Control
                type="text"
                size="sm"
                placeholder="Min instance boot time"
                value={constraint.min_instance_boot_time}
                onChange={(e) =>
                  setConstraint({
                    ...constraint,
                    min_instance_boot_time: e.target.value,
                  })
                }
              />
            </td>
            <td>
              <Button
                title="Add"
                size="sm"
                variant="outline-secondary"
                className="bg-white border no-hover"
                onClick={handleAdd}
              >
                <BsPlus />
              </Button>
            </td>
          </tr>
        </tbody>
      </Table>
    </div>
  );
};

export default BatchConstraintsWidget;
