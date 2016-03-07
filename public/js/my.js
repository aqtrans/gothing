$(document).ready(function(){
  $("#imageup").submit(function(event){
    event.preventDefault();
    $.ajax( {
      url: '/api/image/new',
      type: 'POST',
      data: new FormData( this ),
      processData: false,
      contentType: false,
      success: function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>Image successfully uploaded! <a style='color:#fff' href=/i/"+data.name+"><i class='fa fa-external-link'></i>[Image link]</a><a class=\"close\">&times;</a></div>");
          //$("#imageup")[0].reset();
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Image successfully uploaded!</h2>\
                <p class=\"lead\">Here is the link to your newly uploaded <a href=/i/"+data.name+"><i class='fa fa-external-link'></i>image</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#imageup")[0].reset();
            });          
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Image failed to upload!</h2>\
                <p class=\"lead\">Failed to upload image. Check things and try again.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#imageup")[0].reset();
            });          
        }
      }
    });
  });
  $("#remoteimageup").submit(function(event){
    event.preventDefault();
	  $.post( "/api/image/remote", $( this ).serialize(), function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>Image successfully uploaded! <a style='color:#fff' href=/i/"+data.name+"><i class='fa fa-external-link'></i>[Image link]</a><a class=\"close\">&times;</a></div>");
          //$("#remoteimageup")[0].reset();
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Image successfully uploaded!</h2>\
                <p class=\"lead\">Here is the link to your newly uploaded <a href=/i/"+data.name+"><i class='fa fa-external-link'></i>image</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#remoteimageup")[0].reset();
            });          
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Image failed to upload!</h2>\
                <p class=\"lead\">Failed to upload image. Check things and try again.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#remoteimageup")[0].reset();
            });
        }
	  });
  });  

  $("#fileup").submit(function(event){
    event.preventDefault();
    $.ajax( {
      url: '/api/file/new',
      type: 'POST',
      data: new FormData( this ),
      processData: false,
      contentType: false,
      success: function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>File successfully uploaded! <a style='color:#fff' href=/d/"+data.name+"><i class='fa fa-external-link'></i>[File link]</a><a class=\"close\">&times;</a></div>");
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">File successfully uploaded!</h2>\
                <p class=\"lead\">Here is the link to your <a href=/d/"+data.name+"><i class='fa fa-external-link'></i>new file</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#fileup")[0].reset();
            });
          //$(document).foundation('alert','reflow');
        } else {
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">File failed to upload!</h2>\
                <p class=\"lead\">Failed to upload file. Check things and try again.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#fileup")[0].reset();
            });
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
        }
      }
    });
  });
  $("#remotefileup").submit(function(event){
    event.preventDefault();
    $.post( "/api/file/remote", $( this ).serialize(), function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>File successfully uploaded! <a style='color:#fff' href=/d/"+data.name+"><i class='fa fa-external-link'></i>[File link]</a><a class=\"close\">&times;</a></div>");
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">File successfully uploaded!</h2>\
                <p class=\"lead\">Here is the link to your <a href=/d/"+data.name+"><i class='fa fa-external-link'></i>new file</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#remotefileup")[0].reset();
            });
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Failed upload!</h2>\
                <p class=\"lead\">Failed to upload your file.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#remotefileup")[0].reset();
            });          
        }
    });
  }); 

  $("#shorturl").submit(function(event){
    event.preventDefault();  
    console.log( $( this ).serialize() ); 
    $.post( "/api/shorten/new", $( this ).serialize(), function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>Link successfully shortened! <a style='color:#fff' href="+data.name+"><i class='fa fa-external-link'></i>[Short URL]</a><a class=\"close\">&times;</a></div>");
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Successfully shortened link!</h2>\
                <p class=\"lead\">Here is your new <a href="+data.name+"><i class='fa fa-external-link'></i>short URL</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#shorturl")[0].reset();
            });
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed to shorten URL<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
	           <h2 id=\"modalTitle\">Failed to shorten URL!</h2>\
	           <p class=\"lead\">The given URL was unable to be shortened for some reason :( </p>\
	           <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');          
        }
      });
  });  

  $("#shorturl2").submit(function(event){
    event.preventDefault();  
    console.log( $( this ).serialize() ); 
    $.post( "/api/shorten/new", $( this ).serialize(), function(data){
        if(data.success){
          //$(".alerts").append("<div class=\"alert-box success\" data-alert>Link successfully shortened! <a style='color:#fff' href="+data.name+"><i class='fa fa-external-link'></i>[Short URL]</a><a class=\"close\">&times;</a></div>");
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
                <h2 id=\"modalTitle\">Successfully shortened link!</h2>\
                <p class=\"lead\">Here is your new <a href="+data.name+"><i class='fa fa-external-link'></i>short URL</a>.</p>\
                <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
                //var modal = $(this);
                $("#shorturl2")[0].reset();
            });
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed to shorten URL<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
	           <h2 id=\"modalTitle\">Failed to shorten URL!</h2>\
	           <p class=\"lead\">The given URL was unable to be shortened for some reason :( </p>\
	           <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');          
        }
      });
  });   

  $("a.delete").click(function(event){
    event.preventDefault();
    var link = $( this ).attr("href");
    //console.log( link );
    $.get( link, function(data){
        if(data.success){
          //<div class=\"alert-box success\" data-alert>Successfully deleted "+data.name+"!<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
	           <h2 id=\"modalTitle\">Successfully deleted "+data.name+"!</h2>\
	           <p class=\"lead\">"+data.name+" has been successfully deleted.</p>\
	           <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
            $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
              //var modal = $(this);
              location.reload();
            });
        } else {
          //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed to delete<a class=\"close\">&times;</a></div>");
          //$(document).foundation('alert','reflow');
            $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
	           <h2 id=\"modalTitle\">Failed to delete item!</h2>\
	           <p class=\"lead\">Given item was unable to be deleted for some reason :( </p>\
	           <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
                </div>");
            $('#alertModal').foundation('reveal', 'open');
            $(document).foundation('reveal', 'reflow');
        }
    });    
  });

  $("#login").submit(function(event){
    event.preventDefault();   
    $.post( "login", $( this ).serialize(), function(data){
      if(data.success){
        $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
            <h2 id=\"modalTitle\">Successfully logged in!</h2>\
            <p class=\"lead\">Close this box to be redirected to your previously scheduled program.</p>\
            <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
            </div>");
        $('#alertModal').foundation('reveal', 'open');
        $(document).foundation('reveal', 'reflow');
        $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
            //var modal = $(this);
            $(location).attr('href', data.name);
        });
        //setTimeout(function() {
        //   window.location.href = data.name;
        //}, 2000);  
        //$(location).attr('href', data.name);
      } else {
        //$(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed login<a class=\"close\">&times;</a></div>");
        //$(document).foundation('alert','reflow');
        $(".alerts").append("<div id=\"alertModal\" class=\"reveal-modal\" data-reveal aria-labelledby=\"modalTitle\" aria-hidden=\"true\" role=\"dialog\">\
            <h2 id=\"modalTitle\">Login failure!</h2>\
            <p class=\"lead\">Please check your credentials and try again.</p>\
            <a class=\"close-reveal-modal\" aria-label=\"Close\">&#215;</a>\
            </div>");
        $('#alertModal').foundation('reveal', 'open');
        $(document).foundation('reveal', 'reflow'); 
        //$(location).attr('href', data.name);
        $(document).on('closed.fndtn.reveal', '[data-reveal]', function () {
            //var modal = $(this);
            $("#login")[0].reset();
        });        
      }
    });
  });

});
