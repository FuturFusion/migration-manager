import { FC } from 'react';
import { Table } from 'react-bootstrap';

interface Props {
  headers: string[];
  rows: Object[][];
}

const DataTable: FC<Props> = ({ headers, rows}) => {

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
    const dataRows = rows.map((rowItem, rowIndex) => {
      const row = rowItem.map((item, index) => {
        return (
          <td key={ index }>{ item.toString() }</td>
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
