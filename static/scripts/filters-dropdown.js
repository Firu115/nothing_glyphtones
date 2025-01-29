const form = document.querySelector("form#filters")
const search = form.querySelector("input[name='s']")
const sort = form.querySelector("select[name='o']")
const button = form.querySelector("button#show-checkboxes")
const container = form.querySelector("#filter-container")
const allCheckboxes = container.querySelectorAll(`label>input#effect, label>input#phone`)
const hiddenPhones = form.querySelector("#aggregated-phones")
const hiddenEffects = form.querySelector("#aggregated-effects")

function toggleFiltersVisibility(e) {
    container.classList.toggle("open")
    button.textContent = container.classList.contains("open") ? "Hide Filters" : "Show Filters"
}

function updateHiddenInputs() {
    const phones = []
    const effects = []
    for (const checkbox of allCheckboxes) {
        if (checkbox.checked) {
            if (checkbox.id === "phone") phones.push(checkbox.value)
            if (checkbox.id === "effect") effects.push(checkbox.value)
        }
    }
    hiddenPhones.value = phones.join(",")
    hiddenEffects.value = effects.join(",")
}

function enableAndDisableSort() {
    if (search.value === "") {
        sort.disabled = false
    } else {
        sort.disabled = true
    }
}

button.addEventListener("click", toggleFiltersVisibility)
document.addEventListener("DOMContentLoaded", updateHiddenInputs)
form.addEventListener("change", updateHiddenInputs)
search.addEventListener("input", enableAndDisableSort)
window.addEventListener("load", enableAndDisableSort)