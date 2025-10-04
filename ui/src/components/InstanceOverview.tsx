import { useQuery } from "@tanstack/react-query";
import { Table } from "react-bootstrap";
import { Link, useParams } from "react-router";
import { fetchInstance } from "api/instances";
import InstanceItemOverride from "components/InstanceItemOverride";
import { bytesToHumanReadable, hasOverride } from "util/instance";

const InstanceOverview = () => {
  const { uuid } = useParams<{ uuid: string }>();

  const {
    data: instance,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["instances", uuid],
    queryFn: () => {
      return fetchInstance(uuid ?? "");
    },
  });

  if (isLoading || !instance) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading instances</div>;
  }

  return (
    <>
      <h6 className="mb-3">General</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">UUID</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.uuid}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Source</div>
          <div className="col-10 detail-table-cell"> {instance.source}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Source type</div>
          <div className="col-10 detail-table-cell">
            {" "}
            {instance.source_type}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Location</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.location}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">OS version</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.os_version}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">CPU</div>
          <div className="col-10 detail-table-cell">
            <InstanceItemOverride
              original={instance.properties.cpus}
              override={
                instance.overrides && instance.overrides.properties.cpus
              }
              showOverride={
                hasOverride(instance) && instance.overrides.properties.cpus > 0
              }
            />
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Memory</div>
          <div className="col-10 detail-table-cell">
            <InstanceItemOverride
              original={bytesToHumanReadable(instance.properties.memory)}
              override={bytesToHumanReadable(
                instance.overrides?.properties.memory,
              )}
              showOverride={
                hasOverride(instance) &&
                instance.overrides.properties.memory > 0
              }
            />
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Firmware</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.legacy_boot ? "BIOS" : "UEFI"}
          </div>
        </div>
        {!instance.properties.legacy_boot && (
          <div className="row">
            <div className="col-2 detail-table-header">Secure boot</div>
            <div className="col-10 detail-table-cell">
              {instance.properties.secure_boot ? "Yes" : "No"}
            </div>
          </div>
        )}
        <div className="row">
          <div className="col-2 detail-table-header">Running</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.running ? "Yes" : "No"}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Background import</div>
          <div className="col-10 detail-table-cell">
            {instance.properties.background_import ? "Yes" : "No"}
          </div>
        </div>
      </div>
      {instance.properties.nics?.length > 0 && (
        <>
          <hr className="my-4" />
          <h6 className="mb-3">NICs</h6>
          <div className="container">
            <div className="row">
              <Table borderless size="sm">
                <thead>
                  <tr className="overview-table-header">
                    <th>Id</th>
                    <th>Hardware Address</th>
                    <th>Network</th>
                  </tr>
                </thead>
                <tbody>
                  {instance.properties.nics.map((item, index) => (
                    <tr key={index}>
                      <td>{item.id}</td>
                      <td>{item.hardware_address}</td>
                      <td>
                        <Link
                          to={`/ui/networks/${item.id}?source=${instance.source}`}
                          className="data-table-link"
                        >
                          {item.network}
                        </Link>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </div>
          </div>
        </>
      )}
      {instance.properties.disks?.length > 0 && (
        <>
          <hr className="my-4" />
          <h6 className="mb-3">Disks</h6>
          <div className="container">
            <div className="row">
              <Table borderless size="sm">
                <thead>
                  <tr className="overview-table-header">
                    <th>Name</th>
                    <th>Capacity</th>
                    <th>Shared</th>
                    <th>Supported</th>
                  </tr>
                </thead>
                <tbody>
                  {instance.properties.disks.map((item, index) => (
                    <tr key={index}>
                      <td>{item.name}</td>
                      <td>{bytesToHumanReadable(item.capacity)}</td>
                      <td>{item.shared ? "Yes" : "No"}</td>
                      <td>{item.supported ? "Yes" : "No"}</td>
                    </tr>
                  ))}
                </tbody>
              </Table>
            </div>
          </div>
        </>
      )}
    </>
  );
};

export default InstanceOverview;
