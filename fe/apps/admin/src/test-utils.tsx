import { act } from "react"
import { createRoot, type Root } from "react-dom/client"

export async function render(ui: React.ReactNode) {
  const host = document.createElement("div")
  document.body.append(host)
  let root: Root | undefined
  await act(async () => {
    root = createRoot(host)
    root.render(ui)
  })
  return {
    host,
    unmount: async () => {
      await act(async () => root?.unmount())
      host.remove()
    },
  }
}

export async function submit(form: HTMLFormElement) {
  await act(async () => {
    form.dispatchEvent(
      new SubmitEvent("submit", { bubbles: true, cancelable: true })
    )
  })
}

export async function click(el: Element) {
  await act(async () => {
    el.dispatchEvent(new MouseEvent("click", { bubbles: true }))
  })
}
