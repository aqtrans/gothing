# About

This is my first major programming project, and my first major experience with Golang, so please excuse the messiness and feel free to let me know if there are any major issues!
I started out with the Golang.org Wiki guide, but after finishing that up quicker than I thought, I decided to see what other functionality I could build in...
**Note: I may have gotten carried away. :D**

You can see it in action [here](http://go.jba.io).

# Features
As of 05/09/2015, the doodad has the following features:
- User authentication, using my custom authentication functions, heavily based off [this](http://mschoebel.info/2014/03/09/snippet-golang-webapp-login-logout.html) blog post
- Shorten URLs to subdomains of my es.gy domain, so I can have things like http://ygdr.es.gy/
    - If the 'long URL' is found to point to an image uploaded to the app, the image is directly served 
- Handle image and file uploads from local filesystem and remote URLs
- Separate image gallery listing all uploaded images, utilizing [FreezeFrame](http://freezeframe.chrisantonellis.com/) which pauses GIFs until moused over, to try and avoid GIF-incurred CPU spikes when visited
- Pastebin functionality, with rudimentary XSS (just `<script>` for now) protection
- 'Snippet' functionality separate from pastes, who's main difference is the ability to view in-line, and append things to the page easily
- List all uploaded files, short URLs, and snippets
    - If logged in as the admin user, you're given a Delete button on this page to do just that
- Embiggen GIFs uploaded to the app (using gifsicle, not included), by using either a dedicated 'GifTLD' domain in-place of the configured 'ImageTLD', or a '/big/' subroute in the URL of the uploaded image (http://i.es.gy/dayum.gif -> http://i.es.gy/big/dayum.gif OR http://big.es.gy/dayum.gif)
- 'Looking Glass' functionality, with the ability to ping, traceroute, and perform an MTR to a specified domain or IP


# Configuration

The app expects a file named 'conf.json' in the same directory as the binary, containing the following structure.
I have added comments here for clarity, so if copying and pasting, remove them: 
```
{
    //What port you want the app to listen on
    "Port": "3000",

    //Username of the initial admin user
    "Username": "admin",

    //Password of the initial admin user
    "Password": "admin",

    //Email of the initial admin user
    "Email": "admin@main.tld",

    //Directoy where images should be uploaded to
    "ImgDir": "./up-imgs/",

    //Directory where files should be uploaded to
    "FileDir": "./up-files/",

    //Directory where embiggened GIFs are generated to
    "GifDir": "./big-imgs/",

    //The domain where all functions should be
    "MainTLD": "the.main.tld",

    //The domain where wildcard-subdomains are used for short URLs
    "ShortTLD": "short.tld",

    //The domain where a feature-limited image gallery is served
    "ImageTLD": "image.tld",

    //The domain where all embiggened GIFs can be linked to, for easy linking
    "GifTLD": "gif.tld"
}
```
