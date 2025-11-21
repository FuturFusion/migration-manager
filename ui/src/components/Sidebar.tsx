import { Nav, Navbar } from "react-bootstrap";
import { Link } from "react-router";
import { FaArrowRight, FaArrowLeft } from "react-icons/fa";
import {
  BsArchiveFill,
  BsBox,
  BsStack,
  BsFillDatabaseFill,
} from "react-icons/bs";
import {
  MdDescription,
  MdLogin,
  MdLogout,
  MdOutlineSettings,
  MdWarningAmber,
} from "react-icons/md";
import { PiNetwork } from "react-icons/pi";
import { useAuth } from "context/authContext";

const Sidebar = () => {
  const { isAuthenticated } = useAuth();

  const logout = () => {
    fetch("/oidc/logout").then(() => {
      window.location.href = "/ui/";
    });
  };

  return (
    <>
      {/* Sidebar Navbar */}
      <Navbar bg="dark" variant="dark" className="d-flex flex-column vh-100">
        <Navbar.Brand href="/ui/" style={{ margin: "5px 15px" }}>
          Migration Manager
        </Navbar.Brand>
        {/* Sidebar content */}
        <Nav className="flex-column w-100 flex-grow-1 overflow-auto">
          {isAuthenticated && (
            <>
              <li>
                <Nav.Link as={Link} to="/ui/sources">
                  <FaArrowRight /> Sources
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/targets">
                  <FaArrowLeft /> Targets
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/instances">
                  <BsBox /> Instances
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/networks">
                  <PiNetwork /> Networks
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/batches">
                  <BsStack /> Batches
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/queue">
                  <BsFillDatabaseFill /> Queue
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/artifacts">
                  <BsArchiveFill /> Artifacts
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/warnings">
                  <MdWarningAmber /> Warnings
                </Nav.Link>
              </li>
            </>
          )}
          {!isAuthenticated && (
            <>
              <li>
                <Nav.Link href="/oidc/login">
                  <MdLogin /> Login
                </Nav.Link>
              </li>
            </>
          )}
        </Nav>
        {/* Bottom Element */}
        <Nav className="flex-column w-100 flex-shrink-0 border-top border-secondary pt-2">
          {isAuthenticated && (
            <>
              <li>
                <Nav.Link
                  as={Link}
                  to="https://docs.futurfusion.io/migration-manager/main/"
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <MdDescription /> Documentation
                </Nav.Link>
              </li>
              <li>
                <Nav.Link as={Link} to="/ui/settings">
                  <MdOutlineSettings /> Settings
                </Nav.Link>
              </li>
              <li>
                <Nav.Link
                  onClick={() => {
                    logout();
                  }}
                >
                  <MdLogout /> Logout
                </Nav.Link>
              </li>
            </>
          )}
        </Nav>
      </Navbar>
    </>
  );
};

export default Sidebar;
