// Example: <gif-player src="blah.gif" thumb="blah.png"></gif-player>

'use strict';

// Feature detect
if (!(window.customElements && document.body.attachShadow)) {
  document.querySelector('gif-player').innerHTML = "<b>Your browser doesn't support Shadow DOM and Custom Elements v1.</b>";
}

customElements.define('gif-player', class extends HTMLElement {
    constructor() {
      super(); // always call super() first in the constructor.

      this.playGif = this.playGif.bind(this);

      // Create shadow DOM for the component.
      let shadowRoot = this.attachShadow({mode: 'open'});

      const style = document.createElement('style');
      style.textContent =
        `
        div {
          position: relative;
          width: 400px;
        }
        img {
          width: 400px;
        }
        svg {
          position: absolute;
          width: 100%;
          height: 100%;
        }
      `;
      // attach styles to shadow DOM
      shadowRoot.appendChild(style);

      var play = document.createElement('div');
      play.innerHTML = `
      <svg class="play-button" viewBox="0 0 200 200" alt="Play GIF">
        <circle cx="100" cy="100" r="90" fill="none" stroke-width="15" stroke="#fff"/>
        <polygon points="70, 55 70, 145 145, 100" fill="#fff"/>
      </svg>
      `;
      
      var img = document.createElement('img');
      img.src = this.getAttribute('thumb');

      play.appendChild(img);      
      shadowRoot.appendChild(play);

      this.div = shadowRoot.querySelector('div');
      this.div.addEventListener('click', this.playGif);

      //this.playButton = shadowRoot.querySelector('play-button');
    }

    connectedCallback() {
      //console.log('Custom square element loaded page.');
      this.div.setAttribute('playing', 'false');
      //this.shadowRoot.querySelector('#omg').addEventListener('click', playGif(this));
    }

    disconnectedCallback() {
      //console.log('Custom square element removed from page.');
    }

    adoptedCallback() {
      //console.log('Custom square element moved to new page.');
    }

    attributeChangedCallback(name, oldValue, newValue) {
      //console.log('Custom square element attributes changed.');
      //updateStyle(this);
    }

    playGif(elem) {
      //console.log(elem);
      //console.log(this.div.getAttribute('playing'));
      const shadow = this.shadowRoot;

      if(this.div.getAttribute('playing') === 'true') {
        shadow.querySelector('img').src = this.getAttribute('thumb');
        this.div.setAttribute('playing', 'false');
        shadow.querySelector('svg.play-button').style.display = 'block';
        return;
      }
      if(this.div.getAttribute('playing') === 'false') {
        shadow.querySelector('img').src = this.getAttribute('src');
        this.div.setAttribute('playing', 'true');
        shadow.querySelector('svg.play-button').style.display = 'none';
        return;
      }
    }
});