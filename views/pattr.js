window.Pattr = {
    isExecutingScope: false,
    directives: {
        'p-text': (el, value, modifiers = {}) => {
            let text = String(value);
            
            // Apply trim modifier
            if (modifiers.trim && modifiers.trim.length > 0) {
                const maxLength = parseInt(modifiers.trim[0]) || 100;
                if (text.length > maxLength) {
                    text = text.substring(0, maxLength) + '...';
                }
            }
            
            el.innerText = text;
        },
        'p-html': (el, value, modifiers = {}) => {
            let html = value;
            
            // Apply allow filter first (if present)
            if (modifiers.allow && modifiers.allow.length > 0) {
                const allowedTags = modifiers.allow;
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = html;
                
                // Recursively filter elements
                const filterNode = (node) => {
                    if (node.nodeType === Node.ELEMENT_NODE) {
                        const tagName = node.tagName.toLowerCase();
                        if (!allowedTags.includes(tagName)) {
                            // Replace with text content
                            return document.createTextNode(node.textContent);
                        }
                        // Keep element but filter children
                        const filtered = node.cloneNode(false);
                        Array.from(node.childNodes).forEach(child => {
                            const filteredChild = filterNode(child);
                            if (filteredChild) filtered.appendChild(filteredChild);
                        });
                        return filtered;
                    }
                    return node.cloneNode();
                };
                
                const filtered = document.createElement('div');
                Array.from(tempDiv.childNodes).forEach(child => {
                    const filteredChild = filterNode(child);
                    if (filteredChild) filtered.appendChild(filteredChild);
                });
                html = filtered.innerHTML;
            }
            
            // Apply trim modifier (counts only text, preserves HTML tags)
            if (modifiers.trim && modifiers.trim.length > 0) {
                const maxLength = parseInt(modifiers.trim[0]) || 100;
                const tempDiv = document.createElement('div');
                tempDiv.innerHTML = html;
                
                let charCount = 0;
                let truncated = false;
                
                // Recursively traverse and trim while preserving HTML structure
                const trimNode = (node) => {
                    if (truncated) return null;
                    
                    if (node.nodeType === Node.TEXT_NODE) {
                        const text = node.textContent;
                        const remaining = maxLength - charCount;
                        
                        if (text.length <= remaining) {
                            charCount += text.length;
                            return node.cloneNode();
                        } else {
                            // This text node exceeds limit
                            truncated = true;
                            const trimmedText = text.substring(0, remaining) + '...';
                            return document.createTextNode(trimmedText);
                        }
                    } else if (node.nodeType === Node.ELEMENT_NODE) {
                        const cloned = node.cloneNode(false);
                        for (let child of node.childNodes) {
                            const trimmedChild = trimNode(child);
                            if (trimmedChild) {
                                cloned.appendChild(trimmedChild);
                            }
                            if (truncated) break;
                        }
                        return cloned;
                    }
                    return node.cloneNode();
                };
                
                const result = document.createElement('div');
                for (let child of tempDiv.childNodes) {
                    const trimmedChild = trimNode(child);
                    if (trimmedChild) {
                        result.appendChild(trimmedChild);
                    }
                    if (truncated) break;
                }
                
                html = result.innerHTML;
            }
            
            el.innerHTML = html;
        },
        'p-show': (el, value) => {
            el.style.display = value ? '' : 'none'
        },
        'p-style': (el, value) => {
            if (typeof value === 'string') {
                el.style.cssText = value;
            } else if (typeof value === 'object' && value !== null) {
                Object.assign(el.style, value);
            }
        },
        'p-class': (el, value) => {
            if (typeof value === 'string') {
                el.className = value;
            } else if (Array.isArray(value)) {
                el.className = value.join(' ');
            } else if (typeof value === 'object' && value !== null) {
                // Object format: { className: boolean, ... }
                el.className = Object.keys(value)
                    .filter(key => value[key])
                    .join(' ');
            }
        },
        'p-model': (el, value) => {
            el.value = value
        },
    },
    
    parseDirectiveModifiers(attrName) {
        // Parse: p-html:trim.300:allow.p.h1.h2
        // Returns: { directive: 'p-html', modifiers: { trim: ['300'], allow: ['p', 'h1', 'h2'] } }
        const parts = attrName.split(':');
        const directive = parts[0];
        const modifiers = {};
        
        // Parse each modifier group
        for (let i = 1; i < parts.length; i++) {
            const modParts = parts[i].split('.');
            const modName = modParts[0];
            const modValues = modParts.slice(1);
            modifiers[modName] = modValues;
        }
        
        return { directive, modifiers };
    },

    async start() {
        this.root = document.documentElement;

        // Load root data (props from CMS)
        const rootDataJsonString = document.getElementById("p-root-data")?.textContent;
        let rootData = {};
        try {
            rootData = JSON.parse(rootDataJsonString || '{}');
        } catch (e) {
            console.error("Error parsing root data JSON:", e);
        }

        // Load local data (UI state variables)
        const localDataJsonString = document.getElementById("p-local-data")?.textContent;
        let localData = {};
        try {
            localData = JSON.parse(localDataJsonString || '{}');
        } catch (e) {
            console.error("Error parsing local data JSON:", e);
        }

        // Merge root and local data
        this.rawData = { ...rootData, ...localData };
        
        // Store root data keys for future API saving (only save props, not local vars)
        this.rootDataKeys = Object.keys(rootData);

        this.buildScopeData(this.root, this.rawData);
        this.data = this.observe(this.rawData)
        this.walkDomScoped(this.root, this.data, true);
        this.refreshAllLoops();
    },

    buildScopeData(el, parentData) {
        let currentData = parentData;
        if (el.hasAttribute('p-scope')) {
            const dataId = el.getAttribute('p-id') || 'missing_p-id';
            if (!parentData._p_children) {
                parentData._p_children = {};
            }
            if (!parentData._p_children[dataId]) {
                parentData._p_children[dataId] = {};
            }
            currentData = parentData._p_children[dataId]; 
            currentData._p_scope = el.getAttribute('p-scope');
        }
        let child = el.firstElementChild;
        while (child) {
            this.buildScopeData(child, currentData); 
            child = child.nextElementSibling;
        }
    },

    setForTemplateRecursive(element, template) {
        // Set _forTemplate on this element
        element._forTemplate = template;
        // Set _forTemplate on all descendants
        Array.from(element.children).forEach(child => {
            this.setForTemplateRecursive(child, template);
        });
    },

    refreshAllLoops(el = this.root) {
        // Find all p-for templates and refresh them
        if (el.tagName === 'TEMPLATE' && el.hasAttribute('p-for')) {
            this.handleFor(el, this.data, false);
            return; // Don't recurse into template contents
        }
        
        // Recurse through children
        let child = el.firstElementChild;
        while (child) {
            this.refreshAllLoops(child);
            child = child.nextElementSibling;
        }
    },

    observe(data, parentScope) {
        const localTarget = data;
        let proxyTarget = localTarget;
        if (parentScope) {
            proxyTarget = Object.create(parentScope._p_target || parentScope);
            Object.assign(proxyTarget, localTarget);
        }
        const proxy = new Proxy(proxyTarget, {
            get: (target, key) => {
                // Special handling for _p_target - return the target itself
                if (key === '_p_target') {
                    return target;
                }
                // Always read from the target to get fresh values
                return target[key];
            },
            set: (target, key, value) => {
                target[key] = value;
                // Skip DOM walk if we're executing p-scope statements
                if (!this.isExecutingScope) {
                    this.walkDomScoped(this.root, this.data, false);
                }
                return true;
            }
        });
        return proxy;
    },

    handleFor(template, parentScope, isHydrating) {
        const forExpr = template.getAttribute('p-for');
        
        // Parse: "varPattern of/in expression"
        // Examples: "cat of cats", "[i, cat] of cats.entries()", "{name, age} of users"
        const match = forExpr.match(/^(?:const|let)?\s*(.+?)\s+(of|in)\s+(.+)$/);
        
        if (!match) {
            console.error(`Invalid p-for expression: ${forExpr}`);
            return;
        }
        
        const [, varPattern, operator, iterableExpr] = match;
        
        if (isHydrating) {
            // Store parent scope on template for refresh
            template._scope = parentScope;
            
            // HYDRATION: Check for SSR-rendered elements or render from scratch
            try {
                // Evaluate the iterable expression
                const iterable = eval(`with (parentScope) { (${iterableExpr}) }`);
                
                // Store metadata for refresh
                template._forData = {
                    varPattern,
                    operator,
                    iterableExpr,
                    renderedElements: []
                };
                
                // Check if there are already rendered elements with p-for-key (SSR)
                // Group elements by their p-for-key value
                const existingElementsByKey = {};
                let sibling = template.nextElementSibling;
                while (sibling && sibling.hasAttribute('p-for-key')) {
                    const key = sibling.getAttribute('p-for-key');
                    if (!existingElementsByKey[key]) {
                        existingElementsByKey[key] = [];
                    }
                    existingElementsByKey[key].push(sibling);
                    sibling = sibling.nextElementSibling;
                }
                
                // Iterate through items
                let index = 0;
                for (const item of iterable) {
                    const loopScope = this.createLoopScope(parentScope, varPattern, item);
                    
                    if (existingElementsByKey[index]) {
                        // HYDRATE: Attach scope to existing SSR elements (may be multiple)
                        existingElementsByKey[index].forEach(el => {
                            el._scope = loopScope;
                            // Set _forTemplate on this element and all descendants
                            this.setForTemplateRecursive(el, template);
                            this.walkDomScoped(el, loopScope, true);
                            template._forData.renderedElements.push(el);
                        });
                    } else {
                        // RENDER: No SSR element, create from template
                        const clone = template.content.cloneNode(true);
                        const elements = Array.from(clone.children);
                        
                        elements.forEach(el => {
                            el._scope = loopScope;
                            // Set _forTemplate on this element and all descendants
                            this.setForTemplateRecursive(el, template);
                            this.walkDomScoped(el, loopScope, true);
                            template._forData.renderedElements.push(el);
                        });
                        
                        // Insert into DOM after template or last rendered element
                        const insertAfter = template._forData.renderedElements[template._forData.renderedElements.length - 1] || template;
                        insertAfter.parentNode.insertBefore(clone, insertAfter.nextSibling);
                        template._forData.renderedElements.push(...elements);
                    }
                    
                    index++;
                }
                
                // RECONCILIATION: Remove any SSR elements that don't have matching data
                // (e.g., elements with p-for-key >= array.length)
                Object.keys(existingElementsByKey).forEach(key => {
                    const keyIndex = parseInt(key);
                    if (keyIndex >= index) {
                        // This SSR element doesn't have corresponding data - remove it
                        existingElementsByKey[key].forEach(el => el.remove());
                    }
                });
            } catch (e) {
                console.error(`Error in p-for hydration: ${forExpr}`, e);
            }
        } else {
            // REFRESH: Update items if array changed
            const forData = template._forData;
            if (!forData) return; // Not hydrated yet, skip
            
            try {
                // Use the parentScope passed in (this.data from refreshAllLoops)
                const templateScope = parentScope;
                
                // Use _p_target to read the actual updated values
                const iterable = eval(`with (templateScope._p_target || templateScope) { (${iterableExpr}) }`);
                
                // Simple strategy: remove all and re-render
                // (More efficient diff algorithm can be added later)
                forData.renderedElements.forEach(el => el.remove());
                forData.renderedElements = [];
                
                // Re-render all items
                let index = 0;
                for (const item of iterable) {
                    const clone = template.content.cloneNode(true);
                    const loopScope = this.createLoopScope(templateScope, forData.varPattern, item);
                    
                    const elements = Array.from(clone.children);
                    elements.forEach(el => {
                        el._scope = loopScope;
                        // Set _forTemplate on this element and all descendants
                        this.setForTemplateRecursive(el, template);
                        el.setAttribute('p-for-key', index);
                        // Important: Pass true for isHydrating to register event listeners
                        this.walkDomScoped(el, loopScope, true);
                    });
                    
                    template.parentNode.insertBefore(clone, template.nextSibling);
                    forData.renderedElements.push(...elements);
                    index++;
                }
            } catch (e) {
                console.error(`Error in p-for refresh: ${forExpr}`, e);
            }
        }
    },

    createLoopScope(parentScope, varPattern, item) {
        // Create a plain object with loop variables
        const loopData = {};
        
        // Handle different variable patterns
        varPattern = varPattern.trim();
        
        if (varPattern.startsWith('[')) {
            // Array destructuring: [i, cat] or [key, value]
            const vars = varPattern.slice(1, -1).split(',').map(v => v.trim());
            if (Array.isArray(item)) {
                vars.forEach((v, i) => loopData[v] = item[i]);
            } else {
                // Item is not array, treat as single value
                loopData[vars[0]] = item;
            }
        } else if (varPattern.startsWith('{')) {
            // Object destructuring: {name, age}
            const vars = varPattern.slice(1, -1).split(',').map(v => v.trim());
            vars.forEach(v => {
                // Handle renamed properties: {name: firstName}
                const [key, alias] = v.split(':').map(s => s.trim());
                loopData[alias || key] = item[key];
            });
        } else {
            // Simple variable: cat
            loopData[varPattern] = item;
        }
        
        // Create scope that inherits from parent
        if (!parentScope) {
            console.error('parentScope is undefined in createLoopScope');
            return new Proxy({}, {get: () => undefined, set: () => false});
        }
        
        const parentTarget = parentScope._p_target || parentScope;
        if (!parentTarget || typeof parentTarget !== 'object') {
            console.error('Invalid parentTarget:', parentTarget);
            return new Proxy(loopData, {
                get: (target, key) => target[key],
                set: (target, key, value) => {target[key] = value; return true;}
            });
        }
        
        const loopTarget = Object.create(parentTarget);
        Object.assign(loopTarget, loopData);
        
        // Add _p_target property
        const proxy = new Proxy(loopTarget, {
            get: (target, key) => target[key],
            set: (target, key, value) => {
                // If setting a loop variable (from loopData), set locally
                if (key in loopData) {
                    target[key] = value;
                } else {
                    // Otherwise, write through to parent target
                    parentTarget[key] = value;
                }
                return true;
            }
        });
        proxy._p_target = loopTarget;
        
        return proxy;
    },

    walkDomScoped(el, parentScope, isHydrating = false) {
        let currentScope = parentScope;

        // --- HANDLE p-for LOOPS ---
        if (el.tagName === 'TEMPLATE' && el.hasAttribute('p-for')) {
            this.handleFor(el, parentScope, isHydrating);
            return; // Don't process template children directly
        }

        // --- SCOPE DETERMINATION & CREATION ---
        if (el.hasAttribute('p-scope')) {
            // A. HYDRATION PHASE (One-Time Setup)
            if (isHydrating) {
                const dataId = el.getAttribute('p-id');
                const localRawData = parentScope._p_target._p_children[dataId]; 
                
                // 1. Create new inherited Proxy
                currentScope = this.observe(localRawData, parentScope); 
                
                // 2. Execute p-scope assignments sequentially (e.g., count = count + 5; count = count * 2)
                try {
                    // Split p-scope into individual statements
                    const statements = localRawData._p_scope.split(';').map(s => s.trim()).filter(s => s);
                    
                    // Execute all statements without triggering DOM walks in between
                    this.isExecutingScope = true;
                    statements.forEach(stmt => {
                        eval(`with (currentScope) { ${stmt} }`);
                    });
                    this.isExecutingScope = false;
                } catch (e) {
                    this.isExecutingScope = false;
                    console.error(`Error executing p-scope expression on ${dataId}:`, e);
                }
            } else {
                // B. REFRESH PHASE (Use stored scope)
                currentScope = el._scope;
                
                // If scope wasn't stored during hydration, use parent scope
                if (!currentScope) {
                    currentScope = parentScope;
                } else {
                    // Check if parent values changed - if so, selectively re-execute p-scope
                    const pScopeExpr = el.getAttribute('p-scope');
                    if (pScopeExpr && currentScope._p_target) {
                        // Get parent scope
                        const parentProto = Object.getPrototypeOf(currentScope._p_target);
                        
                        // Track which parent variables changed
                        const changedParentVars = new Set();
                        if (!el._parentSnapshot) {
                            el._parentSnapshot = {};
                        }
                        
                        // Check which specific parent values changed
                        for (let key in parentProto) {
                            if (!key.startsWith('_p_')) {
                                if (el._parentSnapshot[key] !== parentProto[key]) {
                                    changedParentVars.add(key);
                                }
                                el._parentSnapshot[key] = parentProto[key];
                            }
                        }
                        
                        // If any parent changed, re-execute ALL statements
                        // (statements may depend on each other sequentially)
                        if (changedParentVars.size > 0) {
                            try {
                                // Clear child scope's own properties (except internal ones)
                                // so we re-apply transformations from fresh parent values
                                for (let key in currentScope._p_target) {
                                    if (currentScope._p_target.hasOwnProperty(key) && !key.startsWith('_p_')) {
                                        delete currentScope._p_target[key];
                                    }
                                }
                                
                                // Split p-scope into individual statements
                                const statements = pScopeExpr.split(';').map(s => s.trim()).filter(s => s);
                                
                                const tempScope = new Proxy(currentScope._p_target, {
                                    get: (target, key) => {
                                        if (key === '_p_target' || key === '_p_children' || key === '_p_scope') {
                                            return target[key];
                                        }
                                        // Check if value exists in target (set by earlier statements)
                                        // Use hasOwnProperty to avoid looking up prototype chain
                                        if (target.hasOwnProperty(key)) {
                                            return target[key];
                                        }
                                        // Fall back to parent value
                                        return parentProto[key];
                                    },
                                    set: (target, key, value) => {
                                        target[key] = value;
                                        return true;
                                    }
                                });
                                
                                // Execute ALL statements sequentially without triggering DOM walks
                                // We must re-run all because later statements may depend on earlier ones
                                this.isExecutingScope = true;
                                statements.forEach(stmt => {
                                    eval(`with (tempScope) { ${stmt} }`);
                                });
                                this.isExecutingScope = false;
                            } catch (e) {
                                this.isExecutingScope = false;
                                console.error(`Error re-executing p-scope expression:`, e);
                            }
                        }
                    }
                }
            }
            
        }

        // CRITICAL: Store scope reference on ALL elements during Hydration,
        // and rely on it during Refresh.
        if (isHydrating) {
            el._scope = currentScope;
        }
        
        // --- DIRECTIVE EVALUATION ---
        if (currentScope) { // Safety check to prevent the 'undefined' error
            Array.from(el.attributes).forEach(attribute => {
                // 1. Event Listener Registration (Hydration Only)
                if (isHydrating && attribute.name.startsWith('p-on:')) {
                    let event = attribute.name.replace('p-on:', '');
                    el.addEventListener(event, () => {
                        eval(`with (el._scope) { ${attribute.value} }`);
                        
                        // Refresh ALL loops after any event (data may have changed)
                        this.refreshAllLoops();
                    });
                }
                
                // 2. p-model Two-Way Binding Setup (Hydration Only)
                if (isHydrating && attribute.name === 'p-model') {
                    el.addEventListener('input', (e) => {
                        let value;
                        const type = e.target.type;
                        
                        // Convert value based on input type
                        if (type === 'number' || type === 'range') {
                            // Convert to number, handle empty string
                            value = e.target.value === '' ? null : Number(e.target.value);
                        } else if (type === 'checkbox') {
                            value = e.target.checked;
                        } else if (type === 'radio') {
                            // For radio buttons, use the value if checked
                            value = e.target.checked ? e.target.value : undefined;
                        } else {
                            // Text, email, url, search, tel, password, etc. - keep as string
                            value = e.target.value;
                        }
                        
                        // Only update if we have a valid value (skip undefined for unchecked radios)
                        if (value !== undefined) {
                            eval(`with (el._scope) { ${attribute.value} = value }`);
                        }
                    });
                    
                    // For checkbox and radio, also listen to 'change' event for better UX
                    if (el.type === 'checkbox' || el.type === 'radio') {
                        el.addEventListener('change', (e) => {
                            let value;
                            if (el.type === 'checkbox') {
                                value = e.target.checked;
                            } else {
                                value = e.target.checked ? e.target.value : undefined;
                            }
                            if (value !== undefined) {
                                eval(`with (el._scope) { ${attribute.value} = value }`);
                            }
                        });
                    }
                }
                
                // 3. Directive Evaluation (Both Hydration and Refresh)
                // Check if attribute is a directive (with or without modifiers)
                const parsed = this.parseDirectiveModifiers(attribute.name);
                if (Object.keys(this.directives).includes(parsed.directive)) {
                    // Use el._scope if it exists (for loop items), otherwise use currentScope
                    const evalScope = el._scope || currentScope;
                    const value = eval(`with (evalScope) { (${attribute.value}) }`);
                    this.directives[parsed.directive](el, value, parsed.modifiers);
                }
            });
        }

        // --- RECURSION ---
        let child = el.firstElementChild;
        while (child) {
            this.walkDomScoped(child, currentScope, isHydrating);
            child = child.nextElementSibling;
        }
    }

}

window.Pattr.start()
