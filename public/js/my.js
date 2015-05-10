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
          $(".alerts").append("<div class=\"alert-box success\" data-alert>Image successfully uploaded! <a style='color:#fff' href=/i/"+data.name+"><i class='fa fa-external-link'></i>[Image link]</a><a class=\"close\">&times;</a></div>");
          $("#imageup")[0].reset();
        $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
        }
      }
    });
  });
  $("#remoteimageup").submit(function(event){
    event.preventDefault();
	  $.post( "/api/image/remote", $( this ).serialize(), function(data){
        if(data.success){
          $(".alerts").append("<div class=\"alert-box success\" data-alert>Image successfully uploaded! <a style='color:#fff' href=/i/"+data.name+"><i class='fa fa-external-link'></i>[Image link]</a><a class=\"close\">&times;</a></div>");
          $("#remoteimageup")[0].reset();
        $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
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
          $(".alerts").append("<div class=\"alert-box success\" data-alert>File successfully uploaded! <a style='color:#fff' href=/d/"+data.name+"><i class='fa fa-external-link'></i>[File link]</a><a class=\"close\">&times;</a></div>");
          $("#fileup")[0].reset();
          $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
          $(document).foundation('alert','reflow');
        }
      }
    });
  });
  $("#remotefileup").submit(function(event){
    event.preventDefault();
    $.post( "/api/file/remote", $( this ).serialize(), function(data){
        if(data.success){
          $(".alerts").append("<div class=\"alert-box success\" data-alert>File successfully uploaded! <a style='color:#fff' href=/d/"+data.name+"><i class='fa fa-external-link'></i>[File link]</a><a class=\"close\">&times;</a></div>");
          $("#remotefileup")[0].reset();
        $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed upload<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
        }
    });
  }); 

  $("#shorturl").submit(function(event){
    event.preventDefault();  
    console.log( $( this ).serialize() ); 
    $.post( "/api/shorten/new", $( this ).serialize(), function(data){
        if(data.success){
          $(".alerts").append("<div class=\"alert-box success\" data-alert>Link successfully shortened! <a style='color:#fff' href="+data.name+"><i class='fa fa-external-link'></i>[Short URL]</a><a class=\"close\">&times;</a></div>");
          $("#shorturl")[0].reset();
        $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed to shorten URL<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
        }
      });
  });  

  $("a.delete").click(function(event){
    event.preventDefault();
    var link = $( this ).attr("href");
    //console.log( link );
    $.get( link, function(data){
        if(data.success){
          $(".alerts").append("<div class=\"alert-box success\" data-alert>Successfully deleted "+data.name+"!<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
        } else {
          $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed to delete<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
        }
    });    
  });

  $("#login").submit(function(event){
    event.preventDefault();   
    $.post( "login", $( this ).serialize(), function(data){
      if(data.success){
        $(".alerts").append("<div class=\"alert-box success\" data-alert>Successful login<a class=\"close\">&times;</a></div>");
        $("#login-form").remove();
        $(document).foundation('alert','reflow');
      } else {
        $(".alerts").append("<div class=\"alert-box alert\" data-alert>Failed login<a class=\"close\">&times;</a></div>");
        $(document).foundation('alert','reflow');
      }
    });
  });

});
