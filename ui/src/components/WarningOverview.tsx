import { useQuery } from "@tanstack/react-query";
import { useParams } from "react-router";
import { fetchWarning } from "api/warnings";
import { formatDate } from "util/date";

const WarningOverview = () => {
  const { uuid } = useParams();

  const {
    data: warning = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["warnings", uuid],
    queryFn: () => fetchWarning(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error) {
    return <div>Error while loading warning</div>;
  }

  const messagesLength = warning?.messages.length || 0;
  const messages = warning?.messages;

  return (
    <>
      <h6 className="mb-3">General</h6>
      <div className="container">
        <div className="row">
          <div className="col-2 detail-table-header">UUID</div>
          <div className="col-10 detail-table-cell">{warning?.uuid}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Last message</div>
          <div className="col-10 detail-table-cell">
            {messagesLength > 0 ? messages?.[messagesLength - 1] : ""}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Count</div>
          <div className="col-10 detail-table-cell">{warning?.count}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Scope</div>
          <div className="col-10 detail-table-cell">{warning?.scope.scope}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Scope entity</div>
          <div className="col-10 detail-table-cell">
            {warning?.scope.entity}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Scope entity type</div>
          <div className="col-10 detail-table-cell">
            {warning?.scope.entity_type}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Status</div>
          <div className="col-10 detail-table-cell">{warning?.status}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Type</div>
          <div className="col-10 detail-table-cell">{warning?.type}</div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">First seen</div>
          <div className="col-10 detail-table-cell">
            {formatDate(warning?.first_seen_date)}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Last seen</div>
          <div className="col-10 detail-table-cell">
            {formatDate(warning?.last_seen_date)}
          </div>
        </div>
        <div className="row">
          <div className="col-2 detail-table-header">Updated</div>
          <div className="col-10 detail-table-cell">
            {formatDate(warning?.updated_date)}
          </div>
        </div>
      </div>
    </>
  );
};

export default WarningOverview;
