/* Copyright 2021 The Tiyo authors
 *
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

class Page  {
    toolbar  = null;
    sidebar  = null;
    pipeline = null;
    search   = null;

    constructor() {
        this.toolbar = new Toolbar(this, 'nav#top-nav');
        this.sidebar = new Sidebar(this, 'aside#left-col');
        this.search  = new Search(this, 'aside#search');
        this.pipeline = new Pipeline(this, 'div#paper-pipeline-holder');
    }
}

new Page();
