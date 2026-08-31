import { Navigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { fetchSources } from "api/sources";
import { useAuth } from "context/authContext";

const Home = () => {
  const { isAuthenticated } = useAuth();

  const { data: sources = [], isLoading } = useQuery({
    queryKey: ["sources"],
    queryFn: fetchSources,
    enabled: isAuthenticated,
  });

  if (!isAuthenticated) {
    return (
      <>
        <h1>Welcome to Migration Manager</h1>
        <div>
          Please log in using the navigation links on the left to continue.
        </div>
      </>
    );
  }

  if (isLoading) {
    return <div>Loading...</div>;
  }

  return (
    <Navigate to={sources.length > 0 ? "/ui/stats" : "/ui/sources"} replace />
  );
};

export default Home;
