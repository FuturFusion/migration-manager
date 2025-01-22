import { FC, createContext, ReactNode, useRef, useState, useContext } from "react";

interface ContextProps {
  notify: any;
  notification: any;
}

const NotificationContext = createContext<ContextProps>({
  notify: {
    info: () => undefined,
    success: () => undefined,
    error: () => undefined,
  },
  notification: {}
});

export const NotificationProvider: FC<{children: ReactNode}> = ({ children }) => {
  const [notification, setNotification] = useState({message: '', type: 'primary'});
  const timeoutRef = useRef(-1);

  const setupTimeout = () => {
    clearTimeout(timeoutRef.current);
    timeoutRef.current = setTimeout(() => setNotification({message: '', type: 'primary'}), 5000);
  };

  const notify = {
    info: (message: string) => {
      setNotification({ message: message, type: 'primary' });
      setupTimeout();
    },
    success: (message: string) => {
      setNotification({ message: message, type: 'success' });
      setupTimeout();
    },
    error: (message: string) => {
      setNotification({ message: message, type: 'danger' });
      setupTimeout();
    },
  };

  return (
    <NotificationContext.Provider value={{ notify, notification }}>
      {children}
    </NotificationContext.Provider>
  );
};

export const useNotification = () => {
  return useContext(NotificationContext);
};

