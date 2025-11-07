import { FC, useEffect, useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import { BsPlus, BsTrash } from "react-icons/bs";
import { MigrationNetworkPlacement } from "types/batch";
import { Target } from "types/target";
import { IncusNICType } from "util/network";

interface Props {
  targets: Target[];
  value: MigrationNetworkPlacement[];
  onChange: (value: MigrationNetworkPlacement[]) => void;
}

const MigrationNetworkWidget: FC<Props> = ({ targets, value, onChange }) => {
  const [entries, setEntries] = useState<MigrationNetworkPlacement[]>(
    value || [],
  );

  const handleAdd = () => {
    const newValues = [
      ...entries,
      {
        target: "",
        target_project: "",
        network: "",
        nictype: "" as IncusNICType,
        vlan_id: "",
      },
    ];
    setEntries(newValues);
  };

  useEffect(() => {
    setEntries(value || []);
  }, [value]);

  const handleDelete = (index: number) => {
    const updated = entries.filter((_, idx) => idx != index);
    setEntries(updated);
    onChange(updated);
  };

  function updateField(
    obj: MigrationNetworkPlacement,
    field: string,
    value: string,
  ): MigrationNetworkPlacement {
    return { ...obj, [field]: value };
  }

  const handleEdit = (index: number, field: string, value: string) => {
    const currentValue = entries[index];
    const newValue = updateField(currentValue, field, value);

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
                  <div style={{ flex: 1 }}>
                    <Form.Select
                      name="target"
                      size="sm"
                      value={item.target}
                      onChange={(e) =>
                        handleEdit(index, "target", e.target.value)
                      }
                    >
                      <option value="">-- Select an option --</option>
                      {targets.map((option) => (
                        <option key={option.name} value={option.name}>
                          {option.name}
                        </option>
                      ))}
                    </Form.Select>
                  </div>
                  <div style={{ flex: 1 }}>
                    <Form.Control
                      type="text"
                      size="sm"
                      placeholder="Target project"
                      value={item.target_project}
                      onChange={(e) =>
                        handleEdit(index, "target_project", e.target.value)
                      }
                    />
                  </div>
                  <div style={{ flex: 1 }}>
                    <Form.Control
                      type="text"
                      size="sm"
                      placeholder="Network"
                      value={item.network}
                      onChange={(e) =>
                        handleEdit(index, "network", e.target.value)
                      }
                    />
                  </div>
                  <div style={{ flex: 1 }}>
                    <Form.Select
                      name="nictype"
                      size="sm"
                      value={item.nictype}
                      onChange={(e) =>
                        handleEdit(index, "nictype", e.target.value)
                      }
                    >
                      <option value=""></option>
                      {Object.values(IncusNICType).map((value) => (
                        <option key={value} value={value}>
                          {value}
                        </option>
                      ))}
                    </Form.Select>
                  </div>
                  <div style={{ flex: 1 }}>
                    <Form.Control
                      type="text"
                      size="sm"
                      placeholder="VLAN ID"
                      value={item.vlan_id}
                      onChange={(e) =>
                        handleEdit(index, "vlan_id", e.target.value)
                      }
                    />
                  </div>
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

export default MigrationNetworkWidget;
