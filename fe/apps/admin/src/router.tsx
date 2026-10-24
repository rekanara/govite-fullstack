import { createRouter } from "@tanstack/react-router"
import { routeTree } from "./routeTree.gen"
import type { RouterContext } from "./router-context"

export const router = createRouter({
  routeTree,
  context: {
    auth: undefined as unknown as RouterContext["auth"],
  },
})

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router
  }
}
