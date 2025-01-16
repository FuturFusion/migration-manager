import { Nav, Navbar, Collapse } from 'react-bootstrap';
import { Link } from "react-router";
import { FaArrowRight, FaArrowLeft } from 'react-icons/fa';
import { BsBox, BsStack, BsFillDatabaseFill } from "react-icons/bs";

const Sidebar = () => {
  return (
    <div className="bg-dark" style={{ height: '100vh' }}>
      {/* Sidebar Navbar */}
      <Navbar bg="dark" variant="dark" expand="lg" className="flex-column">
        <Navbar.Brand href="/" style={{ margin: '5px 15px' }}>
          Migration manager
        </Navbar.Brand>

        {/* Sidebar content */}
        <Collapse in={ true }>
          <div id="sidebar-collapse" className="w-100">
            <Nav className="flex-column">
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
              <Nav.Link as={Link} to="/ui/batches">
                <BsStack /> Batches
              </Nav.Link>
              </li>
              <li>
              <Nav.Link as={Link} to="/ui/queue">
                <BsFillDatabaseFill /> Queue
              </Nav.Link>
              </li>
            </Nav>
          </div>
        </Collapse>
      </Navbar>
    </div>
  );
};

export default Sidebar;
