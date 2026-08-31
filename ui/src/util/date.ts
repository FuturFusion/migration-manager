export const formatDate = (dateStr: string | undefined): string => {
  if (!dateStr || dateStr === "0001-01-01T00:00:00Z") {
    return "";
  }

  const date = new Date(dateStr);

  const year = date.getFullYear();
  const month = (date.getMonth() + 1).toString().padStart(2, "0"); // Months start from 0
  const day = date.getDate().toString().padStart(2, "0");
  const hours = date.getHours().toString().padStart(2, "0");
  const minutes = date.getMinutes().toString().padStart(2, "0");
  const seconds = date.getSeconds().toString().padStart(2, "0");

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

// formatCountdown returns the duration until the given date, e.g. "2d 3h 15m", or "0m" if it has passed.
export const formatCountdown = (dateStr: string | undefined): string => {
  if (!dateStr || dateStr === "0001-01-01T00:00:00Z") {
    return "";
  }

  const diffMs = Math.max(0, new Date(dateStr).getTime() - Date.now());
  const totalMinutes = Math.round(diffMs / 60000);

  const days = Math.floor(totalMinutes / (24 * 60));
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
  const minutes = totalMinutes % 60;

  const parts = [];
  if (days > 0) {
    parts.push(`${days}d`);
  }

  if (days > 0 || hours > 0) {
    parts.push(`${hours}h`);
  }

  parts.push(`${minutes}m`);

  return parts.join(" ");
};
