import { FC } from "react";
import { RiResetLeftLine } from "react-icons/ri";
import { syncSource } from "api/sources";
import { useNotification } from "context/notificationContext";
import { Source } from "types/source";

interface Props {
  source: Source;
}

const SourceActions: FC<Props> = ({ source }) => {
  const { notify } = useNotification();

  const onSync = () => {
    syncSource(source.name)
      .then((response) => {
        if (response.error_code == 0) {
          notify.success(`Source sync triggered successfully`);
          return;
        }
        notify.error(response.error);
      })
      .catch((e) => {
        notify.error(`Error during source sync: ${e}`);
      });
  };

  const buttonStyle = {
    cursor: "pointer",
    color: "grey",
  };

  return (
    <div className="relative inline-block">
      <div>
        <RiResetLeftLine
          title="Sync source"
          size={22}
          style={buttonStyle}
          onClick={() => onSync()}
        />
      </div>
    </div>
  );
};

export default SourceActions;
