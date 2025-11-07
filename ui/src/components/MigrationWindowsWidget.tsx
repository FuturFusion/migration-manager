import { FC, useEffect, useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import DatePicker from "react-datepicker";
import { BsPlus, BsTrash } from "react-icons/bs";
import { MigrationWindow } from "types/batch";
import { formatDate } from "util/date";

interface Props {
  value: MigrationWindow[];
  onChange: (value: MigrationWindow[]) => void;
}

const MigrationWindowsWidget: FC<Props> = ({ value, onChange }) => {
  const [entries, setEntries] = useState<MigrationWindow[]>(value || []);

  const handleAdd = () => {
    const newValues = [
      ...entries,
      { name: "", start: "", end: "", lockout: "", config: { capacity: 0 } },
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
    obj: MigrationWindow,
    field: string,
    value: string | number,
  ): MigrationWindow {
    return { ...obj, [field]: value };
  }

  function updateConfig(
    obj: MigrationWindow,
    field: string,
    value: string | number,
  ): MigrationWindow {
    return {
      ...obj,
      config: {
        ...obj.config,
        [field]: value,
      },
    };
  }

  const handleEdit = (index: number, field: string, value: string | number) => {
    const configFieldPrefix = "config.";
    const currentValue = entries[index];
    let newValue = null;

    if (!field.startsWith(configFieldPrefix)) {
      newValue = updateField(currentValue, field, value);
    } else {
      newValue = updateConfig(
        currentValue,
        field.slice(configFieldPrefix.length),
        value,
      );
    }

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
                    <Form.Control
                      type="text"
                      size="sm"
                      placeholder="Name"
                      value={item.name}
                      onChange={(e) =>
                        handleEdit(index, "name", e.target.value)
                      }
                    />
                  </div>
                  <div style={{ flex: 2 }}>
                    <DatePicker
                      className="form-control form-control-sm"
                      placeholderText="Start"
                      selected={item.start ? new Date(item.start) : null}
                      onChange={(date) =>
                        handleEdit(
                          index,
                          "start",
                          date ? formatDate(date.toString()) : "",
                        )
                      }
                      showTimeSelect
                      timeFormat="HH:mm"
                      timeIntervals={15}
                      timeCaption="time"
                      dateFormat="yyyy-MM-dd HH:mm:ss"
                    />
                  </div>
                  <div style={{ flex: 2 }}>
                    <DatePicker
                      className="form-control form-control-sm"
                      placeholderText="End"
                      selected={item.end ? new Date(item.end) : null}
                      onChange={(date) =>
                        handleEdit(
                          index,
                          "end",
                          date ? formatDate(date.toString()) : "",
                        )
                      }
                      showTimeSelect
                      timeFormat="HH:mm"
                      timeIntervals={15}
                      timeCaption="time"
                      dateFormat="yyyy-MM-dd HH:mm:ss"
                    />
                  </div>
                  <div style={{ flex: 2 }}>
                    <DatePicker
                      className="form-control form-control-sm"
                      placeholderText="Lockout"
                      selected={item.lockout ? new Date(item.lockout) : null}
                      onChange={(date) =>
                        handleEdit(
                          index,
                          "lockout",
                          date ? formatDate(date.toString()) : "",
                        )
                      }
                      showTimeSelect
                      timeFormat="HH:mm"
                      timeIntervals={15}
                      timeCaption="time"
                      dateFormat="yyyy-MM-dd HH:mm:ss"
                    />
                  </div>
                  <div style={{ flex: 1 }}>
                    <Form.Control
                      type="number"
                      size="sm"
                      placeholder="Capacity"
                      value={item.config?.capacity ?? 0}
                      onChange={(e) =>
                        handleEdit(
                          index,
                          "config.capacity",
                          parseInt(e.target.value),
                        )
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
          {entries.length > 0 && (
            <tr>
              <td className="text-muted small">
                Required format: YYYY-MM-DD HH:MM:SS (e.g.,{" "}
                {formatDate(new Date().toString())})
              </td>
            </tr>
          )}
        </tbody>
      </Table>
    </div>
  );
};

export default MigrationWindowsWidget;
