import { FC, ReactNode, useState } from 'react';
import { Col, Form, Row, Table } from 'react-bootstrap';

interface DataTableRow {
  content: ReactNode | string;
  class?: string;
}

interface Props {
  headers: string[];
  rows: DataTableRow[][];
}

const DataTable: FC<Props> = ({ headers, rows}) => {

  const [currentPage, setCurrentPage ] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(20);

  const totalPages = Math.ceil(rows.length / itemsPerPage);

  const indexOfLastItem = currentPage * itemsPerPage;
  const indexOfFirstItem = indexOfLastItem - itemsPerPage;
  const paginatedData = rows.slice(indexOfFirstItem, indexOfLastItem);

  if (totalPages > 0 && currentPage > totalPages) {
    setCurrentPage(1);
  }

  const handlePageChange = (page: number) => {
    if (page > totalPages) {
      page = totalPages;
    } else if (page < 1) {
      page = 1;
    }

    setCurrentPage(page);
  }

  const generateHeaders = () => {
    const headerRow = headers.map((item, index) => {
      return (
        <th key={ index }>{ item }</th>
      );
    });

    return (
      <tr>
        { headerRow }
      </tr>
    );
  }

  const generateRows = () => {
    const dataRows = paginatedData.map((rowItem, rowIndex) => {
      const row = rowItem.map((item, index) => {
        return (
          <td className={item.class} key={ index }>{ item.content }</td>
        );
      });

      return (
        <tr key={ rowIndex }>{ row }</tr>
      );
    });

    return (
      <>
        { dataRows }
      </>
    );
  }

  return (
    <div className="container mt-4">
      <Row className="justify-content-end">
        <Col xs="auto">
          <Form.Control
            type="number"
            name="currentPage"
            size="sm"
            className="page-control"
            value={ currentPage }
            min={1} max={ totalPages > 0 ? totalPages : 1 }
            onChange={(e) => handlePageChange(Number(e.target.value))}/> of { totalPages > 0 ? totalPages : 1 }
        </Col>
        <Col xs="auto">
          <Form.Select
            size="sm"
            onChange={(e) => setItemsPerPage(Number(e.target.value))}>
            <option value={20}>20</option>
            <option value={50}>50</option>
            <option value={100}>100</option>
          </Form.Select>
        </Col>
      </Row>
      <Table className="data-table" hover>
        <thead>
          { generateHeaders() }
        </thead>
        <tbody>
          { generateRows() }
        </tbody>
      </Table>
    </div>
  );
};

export default DataTable;
