const KEY = 'publicdraw.organiser'

/**
 * The organiser's token, kept for this tab only, in sessionStorage. It never
 * reaches the server render, so organiser controls appear after mounting.
 */
export function useOrganiser() {
  const token = useState('organiser-token', () => '')

  if (import.meta.client && getCurrentInstance()) {
    onMounted(() => {
      token.value ||= sessionStorage.getItem(KEY) ?? ''
    })
  }

  const signIn = (value: string) => {
    token.value = value
    sessionStorage.setItem(KEY, value)
  }
  const signOut = () => {
    token.value = ''
    sessionStorage.removeItem(KEY)
  }

  return { token, signIn, signOut }
}
