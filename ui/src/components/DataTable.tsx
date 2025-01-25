import { FC, ReactNode } from 'react';
import { Table } from 'react-bootstrap';

interface DataTableRow {
  content: ReactNode | string;
  class?: string;
}

interface Props {
  headers: string[];
  rows: DataTableRow[][];
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
