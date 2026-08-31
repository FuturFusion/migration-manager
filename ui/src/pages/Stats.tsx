import { useQuery } from "@tanstack/react-query";
import { Button } from "react-bootstrap";
import { fetchStats } from "api/stats";
import StatsBatchesSummary from "components/StatsBatchesSummary";
import StatsBatchListItem from "components/StatsBatchListItem";
import StatsCollapsibleList from "components/StatsCollapsibleList";
import StatsSourcesSummary from "components/StatsSourcesSummary";
import StatsSourceListItem from "components/StatsSourceListItem";

const Stats = () => {
  const refetchInterval = 10000; // 10 seconds

  const {
    data: stats,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["stats"],
    queryFn: fetchStats,
    refetchInterval: refetchInterval,
  });

  if (isLoading) {
    return <div>Loading stats...</div>;
  }

  if (error || !stats) {
    return <div>Error while loading stats</div>;
  }

  return (
    <div className="d-flex flex-column">
      <div className="mx-2 mx-md-4">
        <div className="row">
          <div className="col-12">
            <Button variant="success" className="float-end invisible">
              Placeholder
            </Button>
          </div>
        </div>
      </div>

      <div className="scroll-container flex-grow-1 mx-2 mx-md-4 pb-5">
        <hr />

        <h6 className="mb-3">Sources ({stats.sources.items.length})</h6>
        <StatsSourcesSummary sources={stats.sources} />
        <StatsCollapsibleList
          count={stats.sources.items.length}
          singular="source"
          plural="sources"
        >
          {stats.sources.items.map((item, index) => (
            <StatsSourceListItem key={item.name} source={item} index={index} />
          ))}
        </StatsCollapsibleList>

        <hr />

        <h6 className="mb-3">Batches ({stats.batches.items.length})</h6>
        <StatsBatchesSummary batches={stats.batches} />
        <StatsCollapsibleList
          count={stats.batches.items.length}
          singular="batch"
          plural="batches"
        >
          {stats.batches.items.map((item, index) => (
            <StatsBatchListItem key={item.name} batch={item} index={index} />
          ))}
        </StatsCollapsibleList>
      </div>
    </div>
  );
};

export default Stats;
