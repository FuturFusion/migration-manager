import Form from "react-bootstrap/Form";
import OverlayTrigger from "react-bootstrap/OverlayTrigger";
import Tooltip from "react-bootstrap/Tooltip";
import { IncusAgentConfig } from "types/incusAgent";
import { enabledIncusAgentFeatures, incusAgentFeatures } from "util/incusAgent";

interface Props {
  value?: IncusAgentConfig | null;
  onChange: (value: IncusAgentConfig | null) => void;
  instance?: boolean;
}

const featureDescriptions = {
  exec: "Run commands in the guest.",
  files: "Transfer files to and from the guest.",
  guestapi: "Allow guest applications to use the Incus guest API.",
  metrics: "Report guest performance metrics.",
  mounts: "Mount shared folders in the guest.",
  "port-forward": "Forward connections to ports in the guest.",
  state: "Report guest OS and network information.",
};

const IncusAgentSettings = ({ value, onChange, instance = false }: Props) => {
  const configured = value?.features != null;

  return (
    <fieldset className="mb-3">
      <legend className="fs-6">Incus agent features</legend>
      <Form.Select
        aria-label="Incus agent configuration"
        value={configured ? "configure" : "inherit"}
        onChange={(event) =>
          onChange(
            event.target.value === "configure"
              ? { features: enabledIncusAgentFeatures() }
              : null,
          )
        }
      >
        <option value="inherit">
          {instance ? "Inherit from batch" : "Default"}
        </option>
        <option value="configure">
          {instance
            ? "Configure for this instance"
            : "Configure for this batch"}
        </option>
      </Form.Select>
      {configured && (
        <div className="ps-3 pt-2">
          {incusAgentFeatures.map((feature) => (
            <div key={feature}>
              <OverlayTrigger
                placement="right"
                overlay={
                  <Tooltip id={`incus-agent-tooltip-${feature}`}>
                    {featureDescriptions[feature]}
                  </Tooltip>
                }
              >
                <span className="d-inline-block">
                  <Form.Check
                    id={`incus-agent-${feature}`}
                    label={feature}
                    checked={value.features?.[feature] === true}
                    onChange={(event) =>
                      onChange({
                        features: Object.fromEntries(
                          incusAgentFeatures.map((key) => [
                            key,
                            key === feature
                              ? event.target.checked
                              : value.features?.[key] === true,
                          ]),
                        ),
                      })
                    }
                  />
                </span>
              </OverlayTrigger>
            </div>
          ))}
        </div>
      )}
    </fieldset>
  );
};

export default IncusAgentSettings;
