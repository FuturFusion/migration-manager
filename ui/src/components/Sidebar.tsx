import { Nav, Navbar, Container } from "react-bootstrap";
import { Link } from "react-router";
import { FaArrowRight, FaArrowLeft } from "react-icons/fa";
import { BsBox, BsStack, BsFillDatabaseFill } from "react-icons/bs";
import { MdLogin, MdLogout, MdWarningAmber } from "react-icons/md";
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
      <Navbar bg="dark" variant="dark" className="flex-column vh-100">
        <Navbar.Brand href="/ui/" style={{ margin: "5px 15px" }}>
          Migration Manager
        </Navbar.Brand>

        {/* Sidebar content */}
        <Container className="flex-column" style={{ padding: "0px" }}>
          <Nav className="flex-column w-100">
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
          <div
            className="w-100"
            style={{ position: "absolute", bottom: "20px" }}
          >
            <Nav className="flex-column">
              {isAuthenticated && (
                <>
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
          </div>
        </Container>
      </Navbar>
    </>
  );
};

export default Sidebar;
