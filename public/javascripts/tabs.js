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
