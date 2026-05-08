type Listener = (value: string) => void

const listeners: Listener[] = []

const getStorageKey = () => {
    return 'period_selection_' + window.location.pathname.split('/')[1]
}

export const periodStore = {
    get() {
        return localStorage.getItem(getStorageKey())
    },

    set(value: string) {
        localStorage.setItem(getStorageKey(), value)

        listeners.forEach(listener => listener(value))
    },

    subscribe(listener: Listener) {
        listeners.push(listener)

        return () => {
            const index = listeners.indexOf(listener)

            if (index !== -1) {
                listeners.splice(index, 1)
            }
        }
    }
}