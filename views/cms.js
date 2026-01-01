const cms_root_fields = JSON.parse(document.getElementById('p-root-data').textContent);
//const cms_local_fields = JSON.parse(document.getElementById('p-local-data').textContent);

function createInputs(obj, container, title) {
    container.appendChild(document.createElement('br'));
    fieldset = document.createElement('fieldset');
    legend = document.createElement('legend');
    legend.textContent=title;
    fieldset.appendChild(legend);
    for (const key in obj) {
        if (Object.hasOwnProperty.call(obj, key)) {
            const input = document.createElement('input');
            input.type = 'text';
            let attribute = "p-model";
            if (typeof obj[key] === 'number') {
                input.type = 'number';
            }
            input.id = key;
            input.name = key;
            input.placeholder = key;
            input.setAttribute(attribute, key);
            const label = document.createElement('label');
            label.htmlFor = key;
            label.textContent = key;
            const div = document.createElement('div').appendChild(label).parentNode;
            fieldset.appendChild(div);
            fieldset.appendChild(input);
            fieldset.appendChild(document.createElement('br'));
            fieldset.appendChild(document.createElement('br'));
        }
    }
    container.appendChild(fieldset);
}
const cms = document.getElementById('plenti_cms');
createInputs(cms_root_fields, cms, "Root Data");
//createInputs(cms_local_fields, cms, "Local Data");

document.getElementById('toggle_plenti_cms').addEventListener('click', function () {
    const menu = document.getElementById('plenti_cms');
    menu.classList.toggle('menu-visible');
});
