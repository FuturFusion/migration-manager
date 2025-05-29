export const formatDate = (dateStr: string | undefined): string => {
  if (!dateStr || dateStr === "0001-01-01T00:00:00Z") {
    return "";
  }

  const date = new Date(dateStr);

  const year = date.getUTCFullYear();
  const month = (date.getUTCMonth() + 1).toString().padStart(2, "0"); // Months start from 0
  const day = date.getUTCDate().toString().padStart(2, "0");
  const hours = date.getUTCHours().toString().padStart(2, "0");
  const minutes = date.getUTCMinutes().toString().padStart(2, "0");
  const seconds = date.getUTCSeconds().toString().padStart(2, "0");

  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds} UTC`;
};

const windowDateRegex =
  /^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01]) (?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?: UTC)?$/;

export const isMigrationWindowDateValid = (dateStr: string): boolean => {
  return windowDateRegex.test(dateStr);
};
