import { Navigate } from 'react-router';
import { useAuth } from 'context/auth';

const Home = () => {
  const { isAuthenticated } = useAuth();

  if (!isAuthenticated) {
    return (
      <>
        <h1>Welcome to Migration Manager</h1>
        <div>Please log in using the navigation links on the left to continue.</div>
      </>
    )
  }

  return <Navigate to={`/ui/sources`} replace={true}/>;
};

export default Home;
