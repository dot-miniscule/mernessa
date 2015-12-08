//----------- TAB SELECTION ------------//
$(function() {
  $('.tab-panels .tabs li').on('click', function() {

    //On mouse click find the next element of tab panels
    //find the closes active one and remove that class
    //set the current panel to active
    var $panel = $(this).closest('.tab-panels');
    $panel.find('.tabs li.active').removeClass('active');
    $(this).addClass('active');

    //find which panel to show
    //The related panel is saved in the list item as an attribute
    var panelToShow = $(this).attr('rel');

    //Hide the current panel
    $panel.find('.panel.active').slideUp(400, showNextPanel);

    //show next panel
    function showNextPanel() {
      $('.panel.active').removeClass('active');

      $('#'+panelToShow).slideDown(400, function() {
        $(this).addClass('active');
      });
    }
  });
});

//---------- SUBMIT BUTTON RELOAD PAGE ----------- //

$(function() {
  $('#submitButton').on('click', function() {
    //On mouse click reload the page
    location.reload();
  });
});


//--------- LOCK/UNLOCK RADIO BUTTON ---------//
//Nested for loop.
//Super gross.
//TO BE FIXED AT A LATER DATE
//FOR NOW IT WORKS
function changeImage() {
  var image = document.getElementsByClassName('lockIcon');
  var radios = document.getElementsByClassName('radios');

  for(var j=0; j<image.length; j++) {
    if(image[j].src.match("unlock")) {
      image[j].src = "/images/lockSmall.png";
      for(var i=0; i<radios.length; i++) {
        radios[i].disabled = true;
      }
    } else {
      image[j].src = "/images/unlockSmall.png" ;
      for(var i=0; i<radios.length; i++) {
        radios[i].disabled = false;
      }
    }
  }
}

//-------- TOOLTIPS -------//
$(document).ready(function() {
  setCookies();
});

//-------- COOKIES --------//
function setCookies() {
  var code = location.search.split('code=')[1];
  if (code !== undefined && code !== "") {
    document.cookie = "code=" + code;
  }
}
