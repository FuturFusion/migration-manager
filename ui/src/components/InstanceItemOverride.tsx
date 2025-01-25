import { FC } from 'react';

interface Props {
  original: string | number;
  override: string | number;
  showOverride: boolean;
}

const InstanceItemOverride: FC<Props> = ({original, override, showOverride}) => {

  const originalClass = showOverride ? 'item-deleted' : '';
  return (
    <>
      <span className={originalClass}>{original}</span>
      { showOverride && (<span style={{marginLeft: '5px'}}>{override}</span>)}
    </>
  );
};

export default InstanceItemOverride;
