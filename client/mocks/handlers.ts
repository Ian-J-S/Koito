import { http, HttpResponse } from "msw";

export const successHandler = http.get("/apis/web/v1/user/apikeys", () => {
  return HttpResponse.json([
    { id: 1, label: "Mock Key", key: "mock-key-value" },
  ]);
});

export const unauthorizedHandler = http.get("/apis/web/v1/user/apikeys", () => {
  return HttpResponse.json({ error: "Unauthorized" }, { status: 401 });
});

export const serverErrorHandler = http.get("/apis/web/v1/user/apikeys", () => {
  return HttpResponse.json({ error: "Internal Server Error" }, { status: 500 });
});