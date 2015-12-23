/* ====================== MAINPAGE  ===================== */
/* THIS IS THE JAVASCRIPT FOR THE MAIN PAGE */

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

//---------- CLEAR SELECTION BUTTON -------------- //
$(function() {
  $('#clearButton').on('click', function() {
    $('.no_change_radios').prop('checked', true);
  });
});

//---------- SUBMIT BUTTON RELOAD PAGE ----------- //
$(function() {
  $('#stateForm').submit(function() {
    $('#submitButton').prop('value', 'Submitting');
    $('#submitButton').prop('disabled', true);
    document.cookie = 'submitting=true';

    // Intercept form submission and redirect back to the original page
    $.post( '/', $('#stateForm').serialize()).done(function( data ) {
      window.location = window.location.href.split('#')[0];
    });
    return false;
  });
});

//-------- SETTING COOKIES -------//
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


/* ====================== VIEW TAGS PAGE  ===================== */
/* THIS IS THE JAVASCRIPT FOR THE VIEW TAGS PAGE */

//------- VIEW TAGS TAG SELECTION ------//
//Function selects/deselects tags based on user input, and adds them to an array
//This array is used to generate search parameters for the page
var tagsToSearchFor = [];

$(function() {
  $('ul#selectTags li').click(function(e) {
    if($(this).hasClass("selected")) {
      $(this).removeClass("selected").addClass("deselected");
      //Remove from the array
      var index = tagsToSearchFor.indexOf($(this).html())
      if(index > -1) {
        tagsToSearchFor.splice(index, 1)
      }
    } 
    else {
      $(this).removeClass("deselected").addClass("selected");
      //Add to the array
      tagsToSearchFor.push($(this).html())
    }

    var html=' ';
    for (var i=0; i<tagsToSearchFor.length; i++) {
      html += tagsToSearchFor[i];
      if(tagsToSearchFor.length>1 && i<tagsToSearchFor.length-1) {
        html += ', ';
      }
    }
    console.log(html)
    $('#selectedTags').html(html);
  });
});

//Pass the JS Array to the webui to request new entries from DB
$(function() {
  $(".viewTags#submitButton").click(function(e) {
    console.log("did a thing")
  });
});