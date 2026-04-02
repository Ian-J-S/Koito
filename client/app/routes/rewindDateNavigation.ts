export function getNextMonth(year: number, month: number) {
  if (month === 12) return { year, month: 0 };
  return { year, month: month + 1 };
}

export function getPrevMonth(year: number, month: number) {
  if (month === 0) return { year, month: 12 };
  return { year, month: month - 1 };
}

export function getNextYear(year: number) {
  return year + 1;
}

export function getPrevYear(year: number) {
  return year - 1;
}

export function isFutureDate(year: number, month: number) {
  return new Date(year, month) > new Date();
}

export function isPrevMonthDisabled(year: number, month: number) {
  return (
    new Date(year, month - 2) > new Date() ||
    (new Date().getFullYear() === year && month === 1)
  );
}

export function isNextMonthDisabled(year: number, month: number) {
  return new Date(year, month) > new Date();
}

export function isPrevYearDisabled(year: number, month: number) {
  return new Date(year - 1, month) > new Date();
}

export function isNextYearDisabled(year: number, month: number) {
  return (
    new Date(year + 1, month - 1) > new Date() ||
    (month === 0 && new Date().getFullYear() === year + 1) ||
    (new Date().getMonth() === month - 1 &&
      new Date().getFullYear() === year + 1)
  );
}