export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "";
export const MAP_STYLE_URL: string | null =
  import.meta.env.VITE_MAP_STYLE_URL ?? null;
export const YANDEX_MAPS_API_KEY =
  import.meta.env.VITE_YANDEX_MAPS_API_KEY ?? "";

console.log(YANDEX_MAPS_API_KEY);
