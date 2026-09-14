import { FC } from "react";
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
  const entries = value || [];

  const handleAdd = () => {
    onChange([...entries, { ...INITIAL_CONSTRAINT }]);
  };

  const handleDelete = (index: number) => {
    onChange(entries.filter((_, idx) => idx != index));
  };

  const handleEdit = (index: number, field: string, value: string | number) => {
    const newValues = entries.map((item, idx) =>
      idx === index ? { ...item, [field]: value } : item,
    );
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
                    placeholder="Name"
                    value={item.name}
                    onChange={(e) => handleEdit(index, "name", e.target.value)}
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    placeholder="Description"
                    value={item.description}
                    onChange={(e) =>
                      handleEdit(index, "description", e.target.value)
                    }
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    placeholder="Include expression"
                    value={item.include_expression}
                    onChange={(e) =>
                      handleEdit(index, "include_expression", e.target.value)
                    }
                  />
                  <Form.Control
                    type="number"
                    size="sm"
                    placeholder="Max concurrent instances"
                    value={item.max_concurrent_instances}
                    onChange={(e) =>
                      handleEdit(
                        index,
                        "max_concurrent_instances",
                        parseInt(e.target.value),
                      )
                    }
                  />
                  <Form.Control
                    type="text"
                    size="sm"
                    placeholder="Min instance boot time"
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
                    className="bg-body border no-hover"
                    onClick={() => handleDelete(index)}
                  >
                    <BsTrash />
                  </Button>
                </td>
              </tr>
            </>
          ))}
          <tr>
            <td>
              <Button
                title="Add"
                size="sm"
                variant="outline-secondary"
                className="bg-body border no-hover"
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
