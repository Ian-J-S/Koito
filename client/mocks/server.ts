import { setupServer } from "msw/node";
import { successHandler } from "./handlers";

export const server = setupServer(successHandler);