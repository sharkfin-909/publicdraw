/**
 * One organiser action at a time, with what it is doing and why it failed:
 * run the action, then whatever refreshes the page, and turn a failure into
 * something worth showing. `busy` names the running action, so a page can
 * disable just that control.
 */
export function useAction() {
  const busy = ref('')
  const error = ref('')

  async function run(name: string, action: () => Promise<unknown>, then?: () => Promise<unknown>) {
    error.value = ''
    busy.value = name
    try {
      await action()
      await then?.()
    } catch (e) {
      error.value = errorText(e)
    } finally {
      busy.value = ''
    }
  }

  return { busy, error, run }
}
