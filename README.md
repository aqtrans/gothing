#About

This is my first major programming project, and my first major experience with Golang, so please excuse the messiness and feel free to let me know if there are any major issues!
I started out with the Golang.org Wiki guide, but since then have ventured more.

#Configuration

The app expects a file named 'conf.json' in the same directory as the binary, containing the following structure.
I have added comments here for clarity, so if copying and pasting, remove them: 
```
{
	//What port you want the app to listen on
	"Port": "3000",

	//Username of the initial admin user
    "Username": "***REMOVED***",

    //Password of the initial admin user
    "Password": "***REMOVED***",

    //Email of the initial admin user
    "Email": "me@jba.io",

    //Directoy where images should be uploaded to
    "ImgDir": "./up-imgs/",

    //Directory where files should be uploaded to
    "FileDir": "./up-files/",

    //Directory where embiggened GIFs are generated to
    "GifDir": "./big-imgs/",

    //The domain where all functions should be
    "MainTLD": "go.jba.io",

    //The domain where wildcard-subdomains are used for short URLs
    "ShortTLD": "es.gy",

    //The domain where a feature-limited image gallery is served
    "ImageTLD": "i.es.gy",

    //The domain where all embiggened GIFs can be linked to, for easy linking
    "GifTLD": "big.es.gy"
}
```